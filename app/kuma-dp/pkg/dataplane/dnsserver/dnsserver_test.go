// +build !windows

package dnsserver

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	kuma_dp "github.com/kumahq/kuma/pkg/config/app/kuma-dp"
)

var _ = Describe("DNS Server", func() {

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

	var outReader *os.File
	var outWriter, errWriter *os.File

	BeforeEach(func() {
		var err error
		outReader, outWriter, err = os.Pipe()
		Expect(err).ToNot(HaveOccurred())
		_, errWriter, err = os.Pipe()
		Expect(err).ToNot(HaveOccurred())
	})

	var stopCh chan struct{}
	var errCh chan error

	BeforeEach(func() {
		stopCh = make(chan struct{})
		errCh = make(chan error)
	})

	Describe("Run(..)", func() {
		It("should generate bootstrap config file and start Envoy", func(done Done) {
			// given
			cfg := kuma_dp.Config{
				DNS: kuma_dp.DNS{
					Enabled:           true,
					CoreDNSPort:       16001,
					CoreDNSEmptyPort:  16002,
					EnvoyDNSPort:      16002,
					PrometheusPort:    16003,
					CoreDNSBinaryPath: filepath.Join("testdata", "binary-mock.exit-0.sh"),
					ConfigDir:         configDir,
				},
			}

			expectedConfigFile := filepath.Join(configDir, "Corefile")

			By("starting a mock DNS Server")
			// when
			dnsServer, err := New(&Opts{
				Config: cfg,
				Stdout: outWriter,
				Stderr: errWriter,
			})
			Expect(err).ToNot(HaveOccurred())
			// and
			go func() {
				errCh <- dnsServer.Start(stopCh)
			}()

			By("waiting for mock DNS Server to complete")
			// then
			Eventually(func() bool {
				select {
				case err := <-errCh:
					Expect(err).ToNot(HaveOccurred())
					return true
				default:
					return false
				}
			}, "5s", "10ms").Should(BeTrue())

			By("closing the write side of the pipe")
			// when
			err = outWriter.Close()
			// then
			Expect(err).ToNot(HaveOccurred())

			By("verifying the output of mock DNS Server")
			// when
			var buf bytes.Buffer
			_, err = buf.ReadFrom(outReader)
			// then
			Expect(err).ToNot(HaveOccurred())
			// and
			Expect(strings.TrimSpace(buf.String())).To(Equal(fmt.Sprintf("-conf %s -quiet", expectedConfigFile)))

			By("verifying the contents DNS Server config file")
			// when
			actual, err := ioutil.ReadFile(expectedConfigFile)
			// then
			Expect(err).ToNot(HaveOccurred())
			// and
			Expect(string(actual)).To(Equal(`.:16001 {
    forward . 127.0.0.1:16002
    alternate NXDOMAIN,SERVFAIL,REFUSED . /etc/resolv.conf
    prometheus localhost:16003
    errors
}

.:16002 {
    template ANY ANY . {
      rcode NXDOMAIN
    }
}`))
			// complete
			close(done)
		}, 10)

		It("should return an error if DNS Server crashes", func(done Done) {
			// given
			cfg := kuma_dp.Config{
				DNS: kuma_dp.DNS{
					Enabled:           true,
					CoreDNSBinaryPath: filepath.Join("testdata", "binary-mock.exit-1.sh"),
					ConfigDir:         configDir,
				},
			}

			By("starting a mock DNS Server")
			// when
			dnsServer, err := New(&Opts{
				Config: cfg,
				Stdout: &bytes.Buffer{},
				Stderr: &bytes.Buffer{},
			})
			// then
			Expect(err).ToNot(HaveOccurred())
			// and
			go func() {
				errCh <- dnsServer.Start(stopCh)
			}()

			By("waiting for mock DNS Server to complete")
			// when
			err = <-errCh
			// then
			Expect(err).To(BeAssignableToTypeOf(&exec.ExitError{}))

			// when
			exitError := err.(*exec.ExitError)
			// then
			Expect(exitError.ProcessState.ExitCode()).To(Equal(1))

			// complete
			close(done)
		}, 10)

		It("should return an error if DNS Server binary path is not found", func(done Done) {
			// given
			cfg := kuma_dp.Config{
				DNS: kuma_dp.DNS{
					Enabled:           true,
					CoreDNSBinaryPath: filepath.Join("testdata"),
					ConfigDir:         configDir,
				},
			}

			By("starting a mock DNS Server")
			// when
			dnsServer, err := New(&Opts{
				Config: cfg,
				Stdout: &bytes.Buffer{},
				Stderr: &bytes.Buffer{},
			})
			// then
			Expect(dnsServer).To(BeNil())
			// and
			Expect(err.Error()).To(ContainSubstring("could not find binary in any of the following paths"))

			// complete
			close(done)
		}, 10)
	})
})
