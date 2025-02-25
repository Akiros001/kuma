package cmd_test

import (
	"io/ioutil"
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	kumactl_resources "github.com/kumahq/kuma/app/kumactl/pkg/resources"

	"github.com/kumahq/kuma/app/kumactl/cmd"
	kumactl_cmd "github.com/kumahq/kuma/app/kumactl/pkg/cmd"
	"github.com/kumahq/kuma/app/kumactl/pkg/config"
)

var _ = Describe("kumactl root cmd", func() {

	var backupDefaultConfigFile string

	BeforeEach(func() {
		file, err := ioutil.TempFile("", "")
		Expect(err).To(Succeed())
		// we have to remove file. Config is created only if file does not exist already
		Expect(os.Remove(file.Name())).To(Succeed())
		backupDefaultConfigFile = config.DefaultConfigFile
		config.DefaultConfigFile = file.Name()
	})

	AfterEach(func() {
		config.DefaultConfigFile = backupDefaultConfigFile
	})

	It("should create default config at startup", func() {
		// given
		rootCtx := &kumactl_cmd.RootContext{
			Runtime: kumactl_cmd.RootRuntime{
				NewAPIServerClient: kumactl_resources.NewAPIServerClient,
			},
		}
		rootCmd := cmd.NewRootCmd(rootCtx)

		// when
		rootCmd.SetArgs([]string{"version"})
		err := rootCmd.Execute()

		// then
		Expect(err).ToNot(HaveOccurred())

		// and default config is created
		expected := `
contexts:
- controlPlane: local
  name: local
controlPlanes:
- coordinates:
    apiServer:
      url: http://localhost:5681
  name: local
currentContext: local
`
		bytes, err := ioutil.ReadFile(config.DefaultConfigFile)
		Expect(err).ToNot(HaveOccurred())
		Expect(bytes).To(MatchYAML(expected))
	})
})
