package main

import (
	"fmt"

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

		It("fails on invalid sample rate", func() {
			for _, sampleRate := range []float32{-0.1, 1.1} {
				config := fmt.Sprintf("---\npatterns:\n- regex: hi\n  sampleRate: %f", sampleRate)
				withConfig(config, func() {
					_, err := NewConfig("logrecycler.yaml")
					Expect(err).ToNot(BeNil())
					expected := fmt.Sprintf("sample must be between 0.0 - 1.0 but was %f", sampleRate)
					Expect(err.Error()).Should(Equal(expected))
				})
			}
		})
	})
})
