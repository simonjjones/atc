package getbuilds_test

import (
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/atc/db"
	dbfakes "github.com/concourse/atc/db/fakes"
	"github.com/concourse/atc/web/getbuilds/fakes"

	. "github.com/concourse/atc/web/getbuilds"
)

var _ = Describe("FetchTemplateData", func() {
	var fakeDB *fakes.FakeBuildsDB
	var fakeConfigDB *dbfakes.FakeConfigDB

	BeforeEach(func() {
		fakeDB = new(fakes.FakeBuildsDB)
		fakeConfigDB = new(dbfakes.FakeConfigDB)
	})

	It("queries the database for a list of all builds", func() {
		builds := []db.Build{
			db.Build{
				ID: 6,
			},
		}

		fakeDB.GetAllBuildsReturns(builds, nil)

		templateData, err := FetchTemplateData(fakeDB, fakeConfigDB)
		Ω(err).ShouldNot(HaveOccurred())

		Ω(templateData.Builds[0].ID).Should(Equal(6))
		Ω(templateData.Builds).Should(BeAssignableToTypeOf([]PresentedBuild{}))
	})

	It("returns an error if fetching from the database fails", func() {
		fakeDB.GetAllBuildsReturns(nil, errors.New("disaster"))

		_, err := FetchTemplateData(fakeDB, fakeConfigDB)
		Ω(err).Should(HaveOccurred())
	})
})
