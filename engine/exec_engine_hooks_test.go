package engine_test

import (
	"os"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/engine"
	"github.com/concourse/atc/engine/fakes"
	"github.com/concourse/atc/exec"
	execfakes "github.com/concourse/atc/exec/fakes"
	"github.com/concourse/atc/worker"
	"github.com/pivotal-golang/lager/lagertest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Exec Engine With Hooks", func() {

	var (
		fakeFactory         *execfakes.FakeFactory
		fakeDelegateFactory *fakes.FakeBuildDelegateFactory
		fakeDB              *fakes.FakeEngineDB

		execEngine engine.Engine

		buildModel db.Build
		logger     *lagertest.TestLogger

		fakeDelegate *fakes.FakeBuildDelegate
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test")

		fakeFactory = new(execfakes.FakeFactory)
		fakeDelegateFactory = new(fakes.FakeBuildDelegateFactory)
		fakeDB = new(fakes.FakeEngineDB)

		execEngine = engine.NewExecEngine(fakeFactory, fakeDelegateFactory, fakeDB)

		fakeDelegate = new(fakes.FakeBuildDelegate)
		fakeDelegateFactory.DelegateReturns(fakeDelegate)

		buildModel = db.Build{ID: 84}
	})

	Context("running hooked composes", func() {
		var (
			taskStepFactory *execfakes.FakeStepFactory
			taskStep        *execfakes.FakeStep

			inputStepFactory *execfakes.FakeStepFactory
			inputStep        *execfakes.FakeStep

			outputStepFactory *execfakes.FakeStepFactory
			outputStep        *execfakes.FakeStep

			dependentStepFactory *execfakes.FakeStepFactory
			dependentStep        *execfakes.FakeStep
		)

		BeforeEach(func() {
			taskStepFactory = new(execfakes.FakeStepFactory)
			taskStep = new(execfakes.FakeStep)
			taskStep.ResultStub = successResult(true)
			taskStepFactory.UsingReturns(taskStep)
			fakeFactory.TaskReturns(taskStepFactory)

			inputStepFactory = new(execfakes.FakeStepFactory)
			inputStep = new(execfakes.FakeStep)
			inputStep.ResultStub = successResult(true)
			inputStepFactory.UsingReturns(inputStep)
			fakeFactory.GetReturns(inputStepFactory)

			outputStepFactory = new(execfakes.FakeStepFactory)
			outputStep = new(execfakes.FakeStep)
			outputStep.ResultStub = successResult(true)
			outputStepFactory.UsingReturns(outputStep)
			fakeFactory.PutReturns(outputStepFactory)

			dependentStepFactory = new(execfakes.FakeStepFactory)
			dependentStep = new(execfakes.FakeStep)
			dependentStep.ResultStub = successResult(true)
			dependentStepFactory.UsingReturns(dependentStep)
			fakeFactory.DependentGetReturns(dependentStepFactory)

			assertNotReleased := func(signals <-chan os.Signal, ready chan<- struct{}) error {
				defer GinkgoRecover()
				Consistently(inputStep.ReleaseCallCount).Should(BeZero())
				Consistently(taskStep.ReleaseCallCount).Should(BeZero())
				Consistently(outputStep.ReleaseCallCount).Should(BeZero())
				return nil
			}

			taskStep.RunStub = assertNotReleased
			inputStep.RunStub = assertNotReleased
			outputStep.RunStub = assertNotReleased
		})

		Context("constructing steps", func() {
			var (
				fakeDelegate          *fakes.FakeBuildDelegate
				fakeInputDelegate     *execfakes.FakeGetDelegate
				fakeExecutionDelegate *execfakes.FakeTaskDelegate
			)

			BeforeEach(func() {
				fakeDelegate = new(fakes.FakeBuildDelegate)
				fakeDelegateFactory.DelegateReturns(fakeDelegate)

				fakeInputDelegate = new(execfakes.FakeGetDelegate)
				fakeDelegate.InputDelegateReturns(fakeInputDelegate)

				fakeExecutionDelegate = new(execfakes.FakeTaskDelegate)
				fakeDelegate.ExecutionDelegateReturns(fakeExecutionDelegate)
			})

			Context("with nested aggregates in hooks", func() {
				BeforeEach(func() {
					plan := atc.Plan{
						Location: &atc.Location{},
						OnSuccess: &atc.OnSuccessPlan{
							Step: atc.Plan{
								Location: &atc.Location{},
								Get: &atc.GetPlan{
									Name: "some-input",
								},
							},
							Next: atc.Plan{
								Location: &atc.Location{},
								Aggregate: &atc.AggregatePlan{
									atc.Plan{
										Location: &atc.Location{},
										OnSuccess: &atc.OnSuccessPlan{
											Step: atc.Plan{
												Location: &atc.Location{
													Hook: "success",
												},
												Task: &atc.TaskPlan{
													Name:   "some-success-task-1",
													Config: &atc.TaskConfig{},
												},
											},
											Next: atc.Plan{
												Location: &atc.Location{
													Hook: "success",
												},
												Get: &atc.GetPlan{
													Name: "some-input",
												},
											},
										},
									},
									atc.Plan{
										Location: &atc.Location{},
										Aggregate: &atc.AggregatePlan{
											atc.Plan{
												Location: &atc.Location{},
												Task: &atc.TaskPlan{
													Name:   "some-success-task-2",
													Config: &atc.TaskConfig{},
												},
											},
										},
									},
									atc.Plan{
										Location: &atc.Location{
											Hook: "success",
										},
										Task: &atc.TaskPlan{
											Name:   "some-success-task-3",
											Config: &atc.TaskConfig{},
										},
									},
								},
							},
						},
					}

					build, err := execEngine.CreateBuild(buildModel, plan)
					Ω(err).ShouldNot(HaveOccurred())
					build.Resume(logger)
				})

				It("constructs the steps correctly", func() {
					Ω(fakeFactory.TaskCallCount()).Should(Equal(3))
					sourceName, workerID, delegate, _, _, _ := fakeFactory.TaskArgsForCall(0)
					Ω(sourceName).Should(Equal(exec.SourceName("some-success-task-1")))
					Ω(workerID).Should(Equal(worker.Identifier{
						BuildID: 84,
						Type:    worker.ContainerTypeTask,
						Name:    "some-success-task-1",
					}))
					Ω(delegate).Should(Equal(fakeExecutionDelegate))

					Ω(fakeFactory.GetCallCount()).Should(Equal(2))
					sourceName, workerID, getDelegate, _, _, _, _ := fakeFactory.GetArgsForCall(1)
					Ω(sourceName).Should(Equal(exec.SourceName("some-input")))
					Ω(workerID).Should(Equal(worker.Identifier{
						BuildID: 84,
						Type:    worker.ContainerTypeGet,
						Name:    "some-input",
					}))

					Ω(getDelegate).Should(Equal(fakeInputDelegate))
					_, _, location := fakeDelegate.InputDelegateArgsForCall(1)
					Ω(location).ShouldNot(BeNil())

					_, _, location = fakeDelegate.ExecutionDelegateArgsForCall(0)
					Ω(location).ShouldNot(BeNil())

					sourceName, workerID, delegate, _, _, _ = fakeFactory.TaskArgsForCall(1)
					Ω(sourceName).Should(Equal(exec.SourceName("some-success-task-2")))
					Ω(workerID).Should(Equal(worker.Identifier{
						BuildID: 84,
						Type:    worker.ContainerTypeTask,
						Name:    "some-success-task-2",
					}))
					Ω(delegate).Should(Equal(fakeExecutionDelegate))

					_, _, location = fakeDelegate.ExecutionDelegateArgsForCall(1)
					Ω(location).ShouldNot(BeNil())

					sourceName, workerID, delegate, _, _, _ = fakeFactory.TaskArgsForCall(2)
					Ω(sourceName).Should(Equal(exec.SourceName("some-success-task-3")))
					Ω(workerID).Should(Equal(worker.Identifier{
						BuildID: 84,
						Type:    worker.ContainerTypeTask,
						Name:    "some-success-task-3",
					}))
					Ω(delegate).Should(Equal(fakeExecutionDelegate))

					_, _, location = fakeDelegate.ExecutionDelegateArgsForCall(2)
					Ω(location).ShouldNot(BeNil())
				})
			})

			Context("with all the hooks", func() {
				BeforeEach(func() {
					plan := atc.Plan{
						Location: &atc.Location{},
						OnSuccess: &atc.OnSuccessPlan{
							Step: atc.Plan{
								Ensure: &atc.EnsurePlan{
									Step: atc.Plan{
										OnSuccess: &atc.OnSuccessPlan{
											Step: atc.Plan{
												OnFailure: &atc.OnFailurePlan{
													Step: atc.Plan{
														Location: &atc.Location{},
														Get: &atc.GetPlan{
															Name: "some-input",
														},
													},
													Next: atc.Plan{
														Location: &atc.Location{
															Hook: "failure",
														},
														Task: &atc.TaskPlan{
															Name:   "some-failure-task",
															Config: &atc.TaskConfig{},
														},
													},
												},
											},
											Next: atc.Plan{
												Location: &atc.Location{
													Hook: "success",
												},
												Task: &atc.TaskPlan{
													Name:   "some-success-task",
													Config: &atc.TaskConfig{},
												},
											},
										},
									},
									Next: atc.Plan{
										Location: &atc.Location{
											Hook: "ensure",
										},
										Task: &atc.TaskPlan{
											Name:   "some-completion-task",
											Config: &atc.TaskConfig{},
										},
									},
								},
							},
							Next: atc.Plan{
								Location: &atc.Location{},
								Task: &atc.TaskPlan{
									Name:   "some-next-task",
									Config: &atc.TaskConfig{},
								},
							},
						},
					}

					build, err := execEngine.CreateBuild(buildModel, plan)
					Ω(err).ShouldNot(HaveOccurred())
					build.Resume(logger)
				})

				It("constructs the step correctly", func() {
					Ω(fakeFactory.GetCallCount()).Should(Equal(1))
					sourceName, workerID, delegate, _, _, _, _ := fakeFactory.GetArgsForCall(0)
					Ω(sourceName).Should(Equal(exec.SourceName("some-input")))
					Ω(workerID).Should(Equal(worker.Identifier{
						BuildID: 84,
						Type:    worker.ContainerTypeGet,
						Name:    "some-input",
					}))

					Ω(delegate).Should(Equal(fakeInputDelegate))
					_, _, location := fakeDelegate.InputDelegateArgsForCall(0)
					Ω(location).ShouldNot(BeNil())
				})

				It("constructs the completion hook correctly", func() {
					Ω(fakeFactory.TaskCallCount()).Should(Equal(4))
					sourceName, workerID, delegate, _, _, _ := fakeFactory.TaskArgsForCall(2)
					Ω(sourceName).Should(Equal(exec.SourceName("some-completion-task")))
					Ω(workerID).Should(Equal(worker.Identifier{
						BuildID: 84,
						Type:    worker.ContainerTypeTask,
						Name:    "some-completion-task",
					}))
					Ω(delegate).Should(Equal(fakeExecutionDelegate))

					_, _, location := fakeDelegate.ExecutionDelegateArgsForCall(2)
					Ω(location).ShouldNot(BeNil())
				})

				It("constructs the failure hook correctly", func() {
					Ω(fakeFactory.TaskCallCount()).Should(Equal(4))
					sourceName, workerID, delegate, _, _, _ := fakeFactory.TaskArgsForCall(0)
					Ω(sourceName).Should(Equal(exec.SourceName("some-failure-task")))
					Ω(workerID).Should(Equal(worker.Identifier{
						BuildID: 84,
						Type:    worker.ContainerTypeTask,
						Name:    "some-failure-task",
					}))
					Ω(delegate).Should(Equal(fakeExecutionDelegate))

					_, _, location := fakeDelegate.ExecutionDelegateArgsForCall(0)
					Ω(location).ShouldNot(BeNil())
				})

				It("constructs the success hook correctly", func() {
					Ω(fakeFactory.TaskCallCount()).Should(Equal(4))
					sourceName, workerID, delegate, _, _, _ := fakeFactory.TaskArgsForCall(1)
					Ω(sourceName).Should(Equal(exec.SourceName("some-success-task")))
					Ω(workerID).Should(Equal(worker.Identifier{
						BuildID: 84,
						Type:    worker.ContainerTypeTask,
						Name:    "some-success-task",
					}))
					Ω(delegate).Should(Equal(fakeExecutionDelegate))

					_, _, location := fakeDelegate.ExecutionDelegateArgsForCall(1)
					Ω(location).ShouldNot(BeNil())
				})

				It("constructs the next step correctly", func() {
					Ω(fakeFactory.TaskCallCount()).Should(Equal(4))
					sourceName, workerID, delegate, _, _, _ := fakeFactory.TaskArgsForCall(3)
					Ω(sourceName).Should(Equal(exec.SourceName("some-next-task")))
					Ω(workerID).Should(Equal(worker.Identifier{
						BuildID: 84,
						Type:    worker.ContainerTypeTask,
						Name:    "some-next-task",
					}))
					Ω(delegate).Should(Equal(fakeExecutionDelegate))
					_, _, location := fakeDelegate.ExecutionDelegateArgsForCall(3)
					Ω(location).ShouldNot(BeNil())

				})
			})
		})

		Context("when the step succeeds", func() {
			BeforeEach(func() {
				inputStep.ResultStub = successResult(true)
			})

			It("runs the next step", func() {
				plan := atc.Plan{
					Location: &atc.Location{},
					OnSuccess: &atc.OnSuccessPlan{
						Step: atc.Plan{
							Location: &atc.Location{},
							Get: &atc.GetPlan{
								Name: "some-input",
							},
						},
						Next: atc.Plan{
							Location: &atc.Location{},
							Task: &atc.TaskPlan{
								Name:   "some-resource",
								Config: &atc.TaskConfig{},
							},
						},
					},
				}

				build, err := execEngine.CreateBuild(buildModel, plan)

				Ω(err).ShouldNot(HaveOccurred())

				build.Resume(logger)

				Ω(inputStep.RunCallCount()).Should(Equal(1))
				Ω(inputStep.ReleaseCallCount()).Should(BeNumerically(">", 0))

				Ω(taskStep.RunCallCount()).Should(Equal(1))
				Ω(taskStep.ReleaseCallCount()).Should(BeNumerically(">", 0))
			})

			It("runs the success hooks, and completion hooks", func() {
				plan := atc.Plan{
					Location: &atc.Location{},
					Ensure: &atc.EnsurePlan{
						Step: atc.Plan{
							OnSuccess: &atc.OnSuccessPlan{
								Step: atc.Plan{
									Location: &atc.Location{},
									Get: &atc.GetPlan{
										Name: "some-input",
									},
								},
								Next: atc.Plan{
									Location: &atc.Location{
										Hook: "success",
									},
									Task: &atc.TaskPlan{
										Name:   "some-resource",
										Config: &atc.TaskConfig{},
									},
								},
							},
						},
						Next: atc.Plan{
							OnSuccess: &atc.OnSuccessPlan{
								Step: atc.Plan{
									Location: &atc.Location{
										Hook: "ensure",
									},
									Put: &atc.PutPlan{
										Name: "some-put",
									},
								},
								Next: atc.Plan{
									Location: &atc.Location{},
									DependentGet: &atc.DependentGetPlan{
										Name: "some-put",
									},
								},
							},
						},
					},
				}

				build, err := execEngine.CreateBuild(buildModel, plan)

				Ω(err).ShouldNot(HaveOccurred())

				build.Resume(logger)

				Ω(inputStep.RunCallCount()).Should(Equal(1))
				Ω(inputStep.ReleaseCallCount()).Should(BeNumerically(">", 0))

				Ω(taskStep.RunCallCount()).Should(Equal(1))
				Ω(taskStep.ReleaseCallCount()).Should(BeNumerically(">", 0))

				Ω(outputStep.RunCallCount()).Should(Equal(1))
				Ω(outputStep.ReleaseCallCount()).Should(BeNumerically(">", 0))

				Ω(dependentStep.RunCallCount()).Should(Equal(1))
				Ω(dependentStep.ReleaseCallCount()).Should(BeNumerically(">", 0))
			})

			Context("when the success hook fails, and has a failure hook", func() {
				BeforeEach(func() {
					taskStep.ResultStub = successResult(false)
				})

				It("does not run the next step", func() {
					plan := atc.Plan{
						Location: &atc.Location{},
						OnSuccess: &atc.OnSuccessPlan{
							Step: atc.Plan{
								OnSuccess: &atc.OnSuccessPlan{
									Step: atc.Plan{
										Location: &atc.Location{},
										Get: &atc.GetPlan{
											Name: "some-input",
										},
									},
									Next: atc.Plan{
										Location: &atc.Location{
											Hook: "success",
										},
										OnFailure: &atc.OnFailurePlan{
											Step: atc.Plan{
												Location: &atc.Location{},
												Task: &atc.TaskPlan{
													Name:   "some-resource",
													Config: &atc.TaskConfig{},
												},
											},
											Next: atc.Plan{
												Location: &atc.Location{
													Hook: "failure",
												},
												Task: &atc.TaskPlan{
													Name:   "some-input-success-failure",
													Config: &atc.TaskConfig{},
												},
											},
										},
									},
								},
							},
							Next: atc.Plan{
								Location: &atc.Location{},
							},
						},
					}

					build, err := execEngine.CreateBuild(buildModel, plan)

					Ω(err).ShouldNot(HaveOccurred())

					build.Resume(logger)

					Ω(inputStep.RunCallCount()).Should(Equal(1))
					Ω(inputStep.ReleaseCallCount()).Should(BeNumerically(">", 0))

					Ω(taskStep.RunCallCount()).Should(Equal(2))
					Ω(inputStep.ReleaseCallCount()).Should(BeNumerically(">", 0))

					Ω(outputStep.RunCallCount()).Should(Equal(0))
					Ω(outputStep.ReleaseCallCount()).Should(Equal(0))

					Ω(dependentStep.RunCallCount()).Should(Equal(0))
					Ω(dependentStep.ReleaseCallCount()).Should(Equal(0))
				})
			})
		})

		Context("when the step fails", func() {
			BeforeEach(func() {
				inputStep.ResultStub = successResult(false)
			})

			It("only run the failure hooks", func() {
				plan := atc.Plan{
					Location: &atc.Location{},
					OnSuccess: &atc.OnSuccessPlan{
						Step: atc.Plan{
							OnFailure: &atc.OnFailurePlan{
								Step: atc.Plan{
									Location: &atc.Location{},
									Get: &atc.GetPlan{
										Name: "some-input",
									},
								},
								Next: atc.Plan{
									Location: &atc.Location{
										Hook: "failure",
									},
									Task: &atc.TaskPlan{
										Name:   "some-resource",
										Config: &atc.TaskConfig{},
									},
								},
							},
						},
						Next: atc.Plan{
							Location: &atc.Location{
								Hook: "success",
							},
						},
					},
				}

				build, err := execEngine.CreateBuild(buildModel, plan)

				Ω(err).ShouldNot(HaveOccurred())

				build.Resume(logger)

				Ω(inputStep.RunCallCount()).Should(Equal(1))
				Ω(inputStep.ReleaseCallCount()).Should(BeNumerically(">", 0))

				Ω(taskStep.RunCallCount()).Should(Equal(1))
				Ω(inputStep.ReleaseCallCount()).Should(BeNumerically(">", 0))

				Ω(outputStep.RunCallCount()).Should(Equal(0))
				Ω(outputStep.ReleaseCallCount()).Should(Equal(0))

				_, cbErr, successful, aborted := fakeDelegate.FinishArgsForCall(0)
				Ω(cbErr).ShouldNot(HaveOccurred())
				Ω(successful).Should(Equal(exec.Success(false)))
				Ω(aborted).Should(BeFalse())
			})
		})

		Context("when a step in the aggregate fails the step fails", func() {
			BeforeEach(func() {
				inputStep.ResultStub = successResult(false)
			})

			It("only run the failure hooks", func() {
				plan := atc.Plan{
					Location: &atc.Location{},
					OnSuccess: &atc.OnSuccessPlan{
						Step: atc.Plan{
							Location: &atc.Location{},
							Aggregate: &atc.AggregatePlan{
								atc.Plan{
									Location: &atc.Location{},
									Task: &atc.TaskPlan{
										Name:   "some-resource",
										Config: &atc.TaskConfig{},
									},
								},
								atc.Plan{
									Location: &atc.Location{},
									OnFailure: &atc.OnFailurePlan{
										Step: atc.Plan{
											Location: &atc.Location{},
											Get: &atc.GetPlan{
												Name: "some-input",
											},
										},
										Next: atc.Plan{
											Location: &atc.Location{
												Hook: "failure",
											},
											Task: &atc.TaskPlan{
												Name:   "some-resource",
												Config: &atc.TaskConfig{},
											},
										},
									},
								},
							},
						},
						Next: atc.Plan{
							Location: &atc.Location{},
						},
					},
				}

				build, err := execEngine.CreateBuild(buildModel, plan)

				Ω(err).ShouldNot(HaveOccurred())

				build.Resume(logger)

				Ω(inputStep.RunCallCount()).Should(Equal(1))

				Ω(taskStep.RunCallCount()).Should(Equal(2))

				Ω(outputStep.RunCallCount()).Should(Equal(0))
			})
		})
	})
})
