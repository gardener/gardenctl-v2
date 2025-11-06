/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package env_test

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
	"text/template"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/gardener/gardenctl-v2/pkg/env"
	"github.com/gardener/gardenctl-v2/pkg/env/testdata"
)

var _ = Describe("Env Commands - Template", func() {
	var (
		out          *bytes.Buffer
		filenames    []string
		t            env.Template
		metadata     map[string]interface{}
		data         map[string]interface{}
		providerType string
		cli          string
		commandPath  string
		shell        string
		prompt       string
		region       string
		targetFlags  string
	)

	BeforeEach(func() {
		out = new(bytes.Buffer)
		providerType = "test"
		commandPath = "gardenctl provider-env"
		shell = "bash"
		prompt = "$ "
		region = "region"
		targetFlags = "--garden garden --project project --shoot shoot"
		metadata = make(map[string]interface{})
		data = make(map[string]interface{})
		data["__meta"] = metadata
	})

	JustBeforeEach(func() {
		t = env.NewTemplate(filenames...)
		metadata["commandPath"] = commandPath
		metadata["cli"] = cli
		metadata["shell"] = shell
		metadata["prompt"] = prompt
		metadata["targetFlags"] = targetFlags
		data["region"] = region
	})

	Describe("parsing the usage-hint template", func() {
		BeforeEach(func() {
			filenames = append(filenames, "helpers")
		})

		DescribeTable("executing the eval-cmd template",
			func(shell string, format string) {
				cmd := fmt.Sprintf("%s %s", commandPath, shell)
				metadata["shell"] = shell
				metadata["cmd"] = cmd
				Expect(t.ExecuteTemplate(out, "eval-cmd", metadata)).To(Succeed())
				Expect(out.String()).To(Equal(fmt.Sprintf(format, cmd)))
			},
			Entry("shell is bash", "bash", "eval $(%s)"),
			Entry("shell is zsh", "zsh", "eval $(%s)"),
			Entry("shell is fish", "fish", "eval (%s)"),
			Entry("shell is powershell", "powershell", "& %s | Invoke-Expression"),
		)

		DescribeTable("executing the usage-hint template",
			func(unset bool, format string) {
				metadata["unset"] = unset
				Expect(t.ExecuteTemplate(out, "usage-hint", metadata)).To(Succeed())
				Expect(out.String()).To(Equal(fmt.Sprintf(format, cli, commandPath, shell)))
			},
			Entry("export environment variables", false, "\n# Run this command to configure %s for your shell:\n# eval $(%s %s)\n"),
			Entry("unset environment variables", true, "\n# Run this command to reset the %s configuration for your shell:\n# eval $(%s -u %s)\n"),
		)
	})

	Describe("parsing the kubernetes template", func() {
		const exportFormat = `export KUBECONFIG='PLACEHOLDER_FILENAME';

# Run this command to configure kubectl for your shell:
# eval $(gardenctl kubectl-env PLACEHOLDER_SHELL)
`

		const unsetFormat = `unset KUBECONFIG;

# Run this command to reset the kubectl configuration for your shell:
# eval $(gardenctl kubectl-env -u PLACEHOLDER_SHELL)
`
		pathToKubeconfig := "/path/to/.kube/config"

		BeforeEach(func() {
			providerType = "kubernetes"
			cli = "kubectl"
			commandPath = "gardenctl kubectl-env"
			filenames = append(filenames, "helpers", providerType)
		})

		JustBeforeEach(func() {
			data["filename"] = pathToKubeconfig
		})

		DescribeTable("executing the bash template",
			func(unset bool, format string) {
				metadata["shell"] = shell
				metadata["unset"] = unset
				Expect(t.ExecuteTemplate(out, shell, data)).To(Succeed())
				expected := strings.NewReplacer(
					"PLACEHOLDER_SHELL", shell,
					"PLACEHOLDER_FILENAME", pathToKubeconfig,
				).Replace(format)
				Expect(out.String()).To(Equal(expected))
			},
			Entry("export environment variables", false, exportFormat),
			Entry("unset environment variables", true, unsetFormat),
		)
	})

	Describe("parsing the gcp template", func() {
		const exportFormat = `export GOOGLE_CREDENTIALS_ACCOUNT=$(< 'PLACEHOLDER_CLIENT_EMAIL_FILE');
export CLOUDSDK_CORE_PROJECT=$(< 'PLACEHOLDER_PROJECT_ID_FILE');
export CLOUDSDK_COMPUTE_REGION=$(< 'PLACEHOLDER_REGION_FILE');
export CLOUDSDK_CONFIG='PLACEHOLDER_CONFIG_DIR';
gcloud auth activate-service-account --key-file 'PLACEHOLDER_CREDENTIALS_FILE' -- "$GOOGLE_CREDENTIALS_ACCOUNT";
rm -f -- 'PLACEHOLDER_CLIENT_EMAIL_FILE';
rm -f -- 'PLACEHOLDER_CREDENTIALS_FILE';
rm -f -- 'PLACEHOLDER_PROJECT_ID_FILE';
rm -f -- 'PLACEHOLDER_REGION_FILE';
printf 'Run the following command to revoke access credentials:\n$ eval $(gardenctl provider-env --garden garden --project project --shoot shoot -u PLACEHOLDER_SHELL)\n';

# Run this command to configure gcloud for your shell:
# eval $(gardenctl provider-env PLACEHOLDER_SHELL)
`

		const unsetFormat = `gcloud auth revoke --verbosity=error -- "$GOOGLE_CREDENTIALS_ACCOUNT";
unset GOOGLE_CREDENTIALS_ACCOUNT;
unset CLOUDSDK_CORE_PROJECT;
unset CLOUDSDK_COMPUTE_REGION;
unset CLOUDSDK_CONFIG;

# Run this command to reset the gcloud configuration for your shell:
# eval $(gardenctl provider-env -u PLACEHOLDER_SHELL)
`
		var (
			clientEmailFile = "/tmp/client_email.txt"
			projectIDFile   = "/tmp/project_id.txt"
			regionFile      = "/tmp/region.txt"
			credentialsFile = "/tmp/credentials.txt"
			configDir       = "config-dir"
		)

		BeforeEach(func() {
			providerType = "gcp"
			cli = "gcloud"
			commandPath = "gardenctl provider-env"
			filenames = append(filenames, "helpers", providerType)
		})

		JustBeforeEach(func() {
			data["dataFiles"] = map[string]interface{}{
				"client_email": clientEmailFile,
				"project_id":   projectIDFile,
				"region":       regionFile,
				"credentials":  credentialsFile,
			}
			data["configDir"] = configDir
		})

		DescribeTable("executing the bash template",
			func(unset bool, format string) {
				metadata["shell"] = shell
				metadata["unset"] = unset
				Expect(t.ExecuteTemplate(out, shell, data)).To(Succeed())
				expected := strings.NewReplacer(
					"PLACEHOLDER_SHELL", shell,
					"PLACEHOLDER_CLIENT_EMAIL_FILE", clientEmailFile,
					"PLACEHOLDER_PROJECT_ID_FILE", projectIDFile,
					"PLACEHOLDER_REGION_FILE", regionFile,
					"PLACEHOLDER_CREDENTIALS_FILE", credentialsFile,
					"PLACEHOLDER_CONFIG_DIR", configDir,
				).Replace(format)
				Expect(out.String()).To(Equal(expected))
			},
			Entry("export environment variables", false, exportFormat),
			Entry("unset environment variables", true, unsetFormat),
		)
	})

	Describe("parsing the azure template", func() {
		const exportFormat = `$Env:AZURE_CLIENT_ID = Get-Content -Raw 'PLACEHOLDER_CLIENT_ID_FILE';
$Env:AZURE_TENANT_ID = Get-Content -Raw 'PLACEHOLDER_TENANT_ID_FILE';
$Env:AZURE_SUBSCRIPTION_ID = Get-Content -Raw 'PLACEHOLDER_SUBSCRIPTION_ID_FILE';
$Env:AZURE_CONFIG_DIR = 'PLACEHOLDER_CONFIG_DIR';
$AZURE_CLIENT_SECRET = Get-Content -Raw 'PLACEHOLDER_CLIENT_SECRET_FILE';
az login --service-principal --username "$Env:AZURE_CLIENT_ID" --password "$AZURE_CLIENT_SECRET" --tenant "$Env:AZURE_TENANT_ID";
Remove-Variable -Name AZURE_CLIENT_SECRET;
az account set --subscription "$Env:AZURE_SUBSCRIPTION_ID";
Remove-Item -Force -ErrorAction SilentlyContinue 'PLACEHOLDER_CLIENT_ID_FILE';
Remove-Item -Force -ErrorAction SilentlyContinue 'PLACEHOLDER_CLIENT_SECRET_FILE';
Remove-Item -Force -ErrorAction SilentlyContinue 'PLACEHOLDER_SUBSCRIPTION_ID_FILE';
Remove-Item -Force -ErrorAction SilentlyContinue 'PLACEHOLDER_TENANT_ID_FILE';
printf 'Run the following command to log out and remove access to Azure subscriptions:\n$ & gardenctl provider-env --garden garden --project project --shoot shoot -u PLACEHOLDER_SHELL | Invoke-Expression\n';
# Run this command to configure az for your shell:
# & gardenctl provider-env PLACEHOLDER_SHELL | Invoke-Expression
`

		const unsetFormat = `az logout --username "$Env:AZURE_CLIENT_ID";
Remove-Item -ErrorAction SilentlyContinue Env:\AZURE_CLIENT_ID;
Remove-Item -ErrorAction SilentlyContinue Env:\AZURE_TENANT_ID;
Remove-Item -ErrorAction SilentlyContinue Env:\AZURE_SUBSCRIPTION_ID;
Remove-Item -ErrorAction SilentlyContinue Env:\AZURE_CONFIG_DIR;
# Run this command to reset the az configuration for your shell:
# & gardenctl provider-env -u PLACEHOLDER_SHELL | Invoke-Expression
`
		var (
			clientIDFile       = "/tmp/clientID.txt"
			clientSecretFile   = "/tmp/clientSecret.txt"
			tenantIDFile       = "/tmp/tenantID.txt"
			subscriptionIDFile = "/tmp/subscriptionID.txt"
			configDir          = "config-dir"
		)

		BeforeEach(func() {
			providerType = "azure"
			cli = "az"
			shell = "powershell"
			commandPath = "gardenctl provider-env"
			filenames = append(filenames, "helpers", providerType)
		})

		JustBeforeEach(func() {
			data["dataFiles"] = map[string]interface{}{
				"clientID":       clientIDFile,
				"clientSecret":   clientSecretFile,
				"tenantID":       tenantIDFile,
				"subscriptionID": subscriptionIDFile,
			}
			data["configDir"] = configDir
		})

		DescribeTable("executing the bash template",
			func(unset bool, format string) {
				metadata["shell"] = shell
				metadata["unset"] = unset
				Expect(t.ExecuteTemplate(out, shell, data)).To(Succeed())
				expected := strings.NewReplacer(
					"PLACEHOLDER_SHELL", shell,
					"PLACEHOLDER_CLIENT_ID_FILE", clientIDFile,
					"PLACEHOLDER_CLIENT_SECRET_FILE", clientSecretFile,
					"PLACEHOLDER_TENANT_ID_FILE", tenantIDFile,
					"PLACEHOLDER_SUBSCRIPTION_ID_FILE", subscriptionIDFile,
					"PLACEHOLDER_CONFIG_DIR", configDir,
				).Replace(format)
				Expect(out.String()).To(Equal(expected))
			},
			Entry("export environment variables", false, exportFormat),
			Entry("unset environment variables", true, unsetFormat),
		)
	})

	Describe("parsing custom templates", func() {
		var filename string

		BeforeEach(func() {
			filenames = append(filenames, "helpers")
		})

		AfterEach(func() {
			removeTempFile(filename)
		})

		Context("and the template does not exists", func() {
			BeforeEach(func() {
				filename = filepath.Join("templates", "invalid.tmpl")
			})

			It("should not find the template", func() {
				Expect(t.ParseFiles(filepath.Join(gardenHomeDir, filename))).To(MatchError(MatchRegexp("^template \\\"invalid\\\" does not exist")))
			})
		})

		Context("and the template has invalid syntax", func() {
			BeforeEach(func() {
				filename = filepath.Join("templates", "invalid.tmpl")
				writeTempFile(filename, readTestFile("templates/invalid.tmpl"))
			})

			It("should fail to parse the template", func() {
				Expect(t.ParseFiles(filepath.Join(gardenHomeDir, filename))).To(MatchError(MatchRegexp("^parsing template \\\"invalid\\\" failed")))
			})
		})

		Context("and the template is valid", func() {
			const exportFormat = `export TEST_TOKEN='PLACEHOLDER_TEST_TOKEN';

# Run this command to configure test for your shell:
# eval $(gardenctl provider-env PLACEHOLDER_SHELL)
`
			var token string

			BeforeEach(func() {
				providerType = "test"
				cli = "test"
				token = "token"
				filename = filepath.Join("templates", providerType+".tmpl")
				writeTempFile(filename, readTestFile("templates/"+providerType+".tmpl"))
				data["unsafeSecretData"] = map[string]interface{}{
					"testToken": token,
				}
			})

			It("should successfully parse the template", func() {
				Expect(t.ParseFiles(filepath.Join(gardenHomeDir, filename))).To(Succeed())
				Expect(t.ExecuteTemplate(out, shell, data)).To(Succeed())
				expected := strings.NewReplacer(
					"PLACEHOLDER_SHELL", shell,
					"PLACEHOLDER_TEST_TOKEN", token,
				).Replace(exportFormat)
				Expect(out.String()).To(Equal(expected))
			})
		})
	})

	Describe("when parsing embedded templates", func() {
		var (
			fsys         = testdata.FS
			textTemplate *template.Template
		)

		BeforeEach(func() {
			textTemplate = template.New("base")
		})

		It("should fail to parse the template", func() {
			Expect(env.ParseFile(fsys, textTemplate, "invalid")).To(MatchError(MatchRegexp("^parsing embedded template \\\"invalid\\\" failed")))
		})

		It("should fail for not existing templates", func() {
			Expect(t.ParseFiles("invalid")).To(MatchError("embedded template \"invalid\" does not exist"))
		})
	})
})
