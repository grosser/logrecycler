package main

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("config", func() {
	Describe("NewConfig", func() {
		It("can parse empty file", func() {
			withConfig("", func() {
				_, err := NewConfig("logrecycler.yaml")
				Expect(err).To(BeNil())
			})
		})

		It("fails when file is not found", func() {
			_, err := NewConfig("whoops")
			Expect(err).ToNot(BeNil())
		})

		It("fails on unknown flags", func() {
			withConfig("wut: true", func() {
				_, err := NewConfig("logrecycler.yaml")
				Expect(err).ToNot(BeNil())
			})
		})
	})
})
