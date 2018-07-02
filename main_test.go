package main_test

import (
	. "github.com/onsi/ginkgo"
)

// https://github.com/cloudfoundry-incubator/gcp-broker-proxy/blob/8a468021be2bf7fe2853479c7a714f30bcc58b92/main_test.go
// assert logging - https://stackoverflow.com/questions/44119951/how-to-check-a-log-output-in-go-test?utm_medium=organic&utm_source=google_rich_qa&utm_campaign=google_rich_qa
// https://onsi.github.io/gomega/#gexec-testing-external-processes
// have a broker also started
// can also validate sigterms
// can also validate cron job via "Eventually"
// can also validate certain stuff is logged
// where to put these tests - here or in lib repo?

// spin up broker serevr, spin up fakeCCServer, assert cron job is executed and apis are called and stuff is logged

var _ = Describe("Main", func() {

})

// enable access for all plans for specific org
//Context("when the org visibility is already present", func() {
//	Context("for some of the plans", func() {
//		It("does not cause any errors", func() {
//
//		})
//	})
//	Context("for all of the plans", func() {
//		It("does not cause any errors", func() {
//
//		})
//	})
//
//})


// enable access for all plans for specific org
//Context("when the org visibility is not present", func() {
//	Context("for some of the plans", func() {
//		It("does not cause any errors", func() {
//
//		})
//	})
//	Context("for all of the plans", func() {
//		It("does not cause any errors", func() {
//
//		})
//	})
//
//})