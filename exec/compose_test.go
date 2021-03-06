package exec_test

import (
	"errors"
	"os"

	. "github.com/concourse/atc/exec"

	"github.com/concourse/atc/exec/fakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/ifrit"
)

var _ = Describe("Compose", func() {
	var (
		fakeStepFactoryA *fakes.FakeStepFactory
		fakeStepFactoryB *fakes.FakeStepFactory

		compose StepFactory

		inStep *fakes.FakeStep
		repo   *SourceRepository

		outStepA *fakes.FakeStep
		outStepB *fakes.FakeStep

		startA  chan error
		finishA chan error

		startB  chan error
		finishB chan error

		step    Step
		process ifrit.Process
	)

	BeforeEach(func() {
		fakeStepFactoryA = new(fakes.FakeStepFactory)
		fakeStepFactoryB = new(fakes.FakeStepFactory)

		compose = Compose(fakeStepFactoryA, fakeStepFactoryB)

		inStep = new(fakes.FakeStep)
		repo = NewSourceRepository()

		outStepA = new(fakes.FakeStep)
		fakeStepFactoryA.UsingReturns(outStepA)

		outStepB = new(fakes.FakeStep)
		fakeStepFactoryB.UsingReturns(outStepB)

		startA = make(chan error, 1)
		finishA = make(chan error, 1)

		startB = make(chan error, 1)
		finishB = make(chan error, 1)

		outStepA.RunStub = func(signals <-chan os.Signal, ready chan<- struct{}) error {
			select {
			case err := <-startA:
				if err != nil {
					return err
				}
			case <-signals:
				return ErrInterrupted
			}

			close(ready)

			select {
			case <-signals:
				return ErrInterrupted
			case err := <-finishA:
				return err
			}
		}

		outStepB.RunStub = func(signals <-chan os.Signal, ready chan<- struct{}) error {
			select {
			case err := <-startB:
				if err != nil {
					return err
				}
			case <-signals:
				return ErrInterrupted
			}

			close(ready)

			select {
			case <-signals:
				return ErrInterrupted
			case err := <-finishB:
				return err
			}
		}
	})

	JustBeforeEach(func() {
		step = compose.Using(inStep, repo)
		process = ifrit.Background(step)
	})

	AfterEach(func() {
		close(startA)
		close(finishA)

		close(startB)
		close(finishB)

		Eventually(process.Wait()).Should(Receive())
	})

	Describe("signalling", func() {
		Context("when the first step is starting", func() {
			It("forwards the signal to the first step and does not continue", func() {
				Consistently(process.Ready()).ShouldNot(Receive())

				process.Signal(os.Interrupt)

				Eventually(process.Wait()).Should(Receive(Equal(ErrInterrupted)))

				Ω(fakeStepFactoryB.UsingCallCount()).Should(BeZero())
			})
		})

		Context("while the first step is running", func() {
			BeforeEach(func() {
				startA <- nil
			})

			It("forwards the signal to the first step and does not continue", func() {
				Consistently(process.Ready()).ShouldNot(BeClosed())

				process.Signal(os.Interrupt)

				Eventually(process.Wait()).Should(Receive(Equal(ErrInterrupted)))

				Ω(fakeStepFactoryB.UsingCallCount()).Should(BeZero())
			})
		})

		Context("while the second step is starting", func() {
			BeforeEach(func() {
				startA <- nil
				finishA <- nil
			})

			It("forwards the signal to the second step", func() {
				Consistently(process.Ready()).ShouldNot(BeClosed())

				Eventually(outStepB.RunCallCount).Should(Equal(1))

				process.Signal(os.Interrupt)

				Eventually(process.Wait()).Should(Receive(Equal(ErrInterrupted)))
			})
		})

		Context("while the second step is running", func() {
			BeforeEach(func() {
				startA <- nil
				finishA <- nil

				startB <- nil
			})

			It("forwards the signal to the second step", func() {
				Eventually(process.Ready()).Should(BeClosed())

				Eventually(outStepB.RunCallCount).Should(Equal(1))

				Consistently(process.Wait()).ShouldNot(Receive())

				process.Signal(os.Interrupt)

				Eventually(process.Wait()).Should(Receive(Equal(ErrInterrupted)))
			})
		})
	})

	Context("when the first source exits successfully", func() {
		BeforeEach(func() {
			startA <- nil
			finishA <- nil
		})

		It("uses the input source for the first step", func() {
			Eventually(fakeStepFactoryA.UsingCallCount).Should(Equal(1))
			step, repo := fakeStepFactoryA.UsingArgsForCall(0)
			Ω(step).Should(Equal(inStep))
			Ω(repo).Should(Equal(repo))
		})

		It("uses the first step's source as the input for the second step", func() {
			Eventually(fakeStepFactoryB.UsingCallCount).Should(Equal(1))
			step, repo := fakeStepFactoryB.UsingArgsForCall(0)
			Ω(step).Should(Equal(outStepA))
			Ω(repo).Should(Equal(repo))
		})

		Context("and the second source exits successfully", func() {
			BeforeEach(func() {
				startB <- nil
				finishB <- nil
			})

			It("exits successfully", func() {
				Eventually(process.Wait()).Should(Receive(BeNil()))
			})

			Describe("releasing", func() {
				It("releases both sources", func() {
					Eventually(process.Wait()).Should(Receive(BeNil()))
					step.Release()
					Ω(outStepA.ReleaseCallCount()).Should(Equal(1))
					Ω(outStepB.ReleaseCallCount()).Should(Equal(1))
				})
			})

			Describe("getting the result", func() {
				BeforeEach(func() {
					outStepB.ResultStub = successResult(true)
				})

				It("delegates to the second source", func() {
					Eventually(process.Wait()).Should(Receive(BeNil()))

					var success Success
					Ω(step.Result(&success)).Should(BeTrue())
					Ω(bool(success)).Should(BeTrue())
				})
			})
		})

		Context("and the second source exits with an error", func() {
			disaster := errors.New("nope")

			BeforeEach(func() {
				startB <- nil
				finishB <- disaster
			})

			It("exits with its error result", func() {
				Eventually(process.Wait()).Should(Receive(Equal(disaster)))
			})

			Describe("releasing", func() {
				It("releases both sources", func() {
					Eventually(process.Wait()).Should(Receive())

					step.Release()
					Ω(outStepA.ReleaseCallCount()).Should(Equal(1))
					Ω(outStepB.ReleaseCallCount()).Should(Equal(1))
				})
			})
		})

		Context("and the second source fails to start", func() {
			disaster := errors.New("nope")

			BeforeEach(func() {
				startB <- disaster
			})

			It("exits with its error result", func() {
				Eventually(process.Wait()).Should(Receive(Equal(disaster)))
			})

			Describe("releasing", func() {
				It("releases both sources", func() {
					Eventually(process.Wait()).Should(Receive())

					step.Release()
					Ω(outStepA.ReleaseCallCount()).Should(Equal(1))
					Ω(outStepB.ReleaseCallCount()).Should(Equal(1))
				})
			})
		})
	})

	Context("when the first source fails to start", func() {
		disaster := errors.New("nope")

		BeforeEach(func() {
			startA <- disaster
		})

		It("exits with its error result", func() {
			Eventually(process.Wait()).Should(Receive(Equal(disaster)))
		})

		It("does not proceed to the second step", func() {
			Ω(fakeStepFactoryB.UsingCallCount()).Should(BeZero())
		})

		Describe("releasing", func() {
			It("releases the first source", func() {
				Eventually(process.Wait()).Should(Receive())

				step.Release()
				Ω(outStepA.ReleaseCallCount()).Should(Equal(1))
				Ω(outStepB.ReleaseCallCount()).Should(BeZero())
			})
		})
	})

	Context("when the first source exits with an error", func() {
		disaster := errors.New("nope")

		BeforeEach(func() {
			startA <- nil
			finishA <- disaster
		})

		It("exits with its error result", func() {
			Eventually(process.Wait()).Should(Receive(Equal(disaster)))
		})

		It("does not proceed to the second step", func() {
			Ω(fakeStepFactoryB.UsingCallCount()).Should(BeZero())
		})

		Describe("releasing", func() {
			It("releases the first source", func() {
				Eventually(process.Wait()).Should(Receive())

				step.Release()
				Ω(outStepA.ReleaseCallCount()).Should(Equal(1))
				Ω(outStepB.ReleaseCallCount()).Should(BeZero())
			})
		})
	})
})
