package twitter_test

import (
	"fmt"
	"strings"

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

		AssertLocationsParsedWithPrefix := func(prefix string) {
			var _a, _b string

			BeforeEach(func() {
				text = prefix + text
			})

			JustBeforeEach(func() {
				_a, _b = ExtractLocationStrings(text)
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

				Context("with text after question mark", func() {
					BeforeEach(func() {
						text += " text after question mark"
					})
					AssertCorrectStrings()
				})
			})

			Context("with trailing whitespace", func() {
				BeforeEach(func() {
					text += " "
				})
				AssertCorrectStrings()
			})

			Context("with trailing hashtag phrases", func() {
				BeforeEach(func() {
					text += " #thanks"
				})
				AssertCorrectStrings()
			})

			Context("with escaped &lt and &gt characters", func() {
				BeforeEach(func() {
					text = strings.Replace(text, "<", "&lt", -1)
					text = strings.Replace(text, ">", "&gt", -1)
				})
				AssertCorrectStrings()
			})
		}

		AssertLocationsParsed := func() {
			AssertLocationsParsedWithPrefix("")
			AssertLocationsParsedWithPrefix("@user ")
			AssertLocationsParsedWithPrefix("@user @user2 ")
		}

		Context("formatted: a -> b", func() {
			BeforeEach(func() {
				text = fmt.Sprintf("%s -> %s", a, b)
			})
			AssertLocationsParsed()
		})

		Context("formatted: a->b", func() {
			BeforeEach(func() {
				text = fmt.Sprintf("%s->%s", a, b)
			})
			AssertLocationsParsed()
		})

		Context("formatted: a to b", func() {
			BeforeEach(func() {
				text = fmt.Sprintf("%s to %s", a, b)
			})
			AssertLocationsParsed()
		})

		Context("formatted with region commas", func() {
			BeforeEach(func() {
				a += ", CA"
				b += ", CA"
				text = fmt.Sprintf("%s to %s", a, b)
			})
			AssertLocationsParsed()
		})

		Context("without first location", func() {
			BeforeEach(func() {
				a = ""
			})

			Context("using ->", func() {
				BeforeEach(func() {
					text = fmt.Sprintf("-> %s", b)
				})
				AssertLocationsParsed()
			})
		})
	})

})
