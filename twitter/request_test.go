package twitter_test

import (
	"fmt"

	"github.com/ChimeraCoder/anaconda"
	. "github.com/mrap/tufro/twitter"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Request", func() {

	Describe("Parsing tweet for location strings", func() {
		var (
			a, b, text string
		)

		BeforeEach(func() {
			a = "San Franisco"
			b = "Los Angeles"
		})

		AssertLocationsParsed := func() {
			var _a, _b string

			JustBeforeEach(func() {
				tweet := &anaconda.Tweet{
					Text: text,
				}
				_a, _b = ExtractLocationStrings(tweet)
			})

			AssertCorrectStrings := func() {
				It("should extract the 'a' string", func() {
					Ω(_a).Should(BeEquivalentTo(a))
				})

				It("should extract the 'b' string", func() {
					Ω(_b).Should(BeEquivalentTo(b))
				})
			}

			Context("basic tweet", func() {
				AssertCorrectStrings()
			})

			Context("with a question mark appended to the end", func() {
				BeforeEach(func() {
					text += "?"
				})
				AssertCorrectStrings()
			})

			Context("with trailing whitespace", func() {
				BeforeEach(func() {
					text += " "
				})
				AssertCorrectStrings()
			})
		}

		Context("formatted: a -> b", func() {
			BeforeEach(func() {
				text = fmt.Sprintf("@user %s -> %s", a, b)
			})
			AssertLocationsParsed()
		})

		Context("formatted: a->b", func() {
			BeforeEach(func() {
				text = fmt.Sprintf("@user %s->%s", a, b)
			})
			AssertLocationsParsed()
		})

		Context("formatted: a to b", func() {
			BeforeEach(func() {
				text = fmt.Sprintf("@user %s to %s", a, b)
			})
			AssertLocationsParsed()
		})
	})

})
