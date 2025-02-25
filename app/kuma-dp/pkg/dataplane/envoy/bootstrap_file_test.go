package envoy

import (
	"io/ioutil"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	kuma_dp "github.com/kumahq/kuma/pkg/config/app/kuma-dp"
)

var _ = Describe("Bootstrap File", func() {
	Describe("GenerateBootstrapFile(..)", func() {

		var configDir string

		BeforeEach(func() {
			var err error
			configDir, err = ioutil.TempDir("", "")
			Expect(err).ToNot(HaveOccurred())
		})
		AfterEach(func() {
			if configDir != "" {
				// when
				err := os.RemoveAll(configDir)
				// then
				Expect(err).ToNot(HaveOccurred())
			}
		})

		It("should create Envoy bootstrap file on disk", func() {
			// given
			config := `node:
  id: example
`
			// and
			runtime := kuma_dp.DataplaneRuntime{
				ConfigDir: configDir,
			}

			// when
			filename, err := GenerateBootstrapFile(runtime, []byte(config))
			// then
			Expect(err).ToNot(HaveOccurred())
			// and
			Expect(filename).To(Equal(filepath.Join(configDir, "bootstrap.yaml")))

			// when
			actual, err := ioutil.ReadFile(filename)
			// then
			Expect(err).ToNot(HaveOccurred())
			// and
			Expect(actual).To(MatchYAML(`
            node:
              id: example
`))
		})
	})
})
