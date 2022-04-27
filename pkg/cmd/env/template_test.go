/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package env_test

import (
	"bytes"
	"fmt"
	"path/filepath"
	"text/template"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	"github.com/gardener/gardenctl-v2/pkg/cmd/env"
	"github.com/gardener/gardenctl-v2/pkg/cmd/env/testdata"
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
		cli = env.GetProviderCLI(providerType)
		metadata["commandPath"] = commandPath
		metadata["cli"] = env.GetProviderCLI(providerType)
		metadata["shell"] = shell
		metadata["prompt"] = prompt
		metadata["targetFlags"] = targetFlags
		data["region"] = region
	})

	Describe("parsing the usage-hint template", func() {
		BeforeEach(func() {
			filenames = append(filenames, "usage-hint")
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
		const exportFormat = `export KUBECONFIG='%[2]s';

# Run this command to configure kubectl for your shell:
# eval $(gardenctl kubectl-env %[1]s)
`

		const unsetFormat = `unset KUBECONFIG;

# Run this command to reset the kubectl configuration for your shell:
# eval $(gardenctl kubectl-env -u %[1]s)
`
		var pathToKubeconfig = "/path/to/.kube/config"

		BeforeEach(func() {
			providerType = "kubernetes"
			commandPath = "gardenctl kubectl-env"
			filenames = append(filenames, "usage-hint", providerType)
		})

		JustBeforeEach(func() {
			data["filename"] = pathToKubeconfig
		})

		DescribeTable("executing the bash template",
			func(unset bool, format string) {
				metadata["shell"] = shell
				metadata["unset"] = unset
				Expect(t.ExecuteTemplate(out, shell, data)).To(Succeed())
				Expect(out.String()).To(Equal(fmt.Sprintf(format, shell, pathToKubeconfig)))
			},
			Entry("export environment variables", false, exportFormat),
			Entry("unset environment variables", true, unsetFormat),
		)
	})

	Describe("parsing the gcp template", func() {
		const exportFormat = `export GOOGLE_CREDENTIALS='{"client_email":"%[3]s","project_id":"%[4]s"}';
export GOOGLE_CREDENTIALS_ACCOUNT='%[3]s';
export CLOUDSDK_CORE_PROJECT='%[4]s';
export CLOUDSDK_COMPUTE_REGION='%[2]s';
export CLOUDSDK_CONFIG='%[5]s';
gcloud auth activate-service-account $GOOGLE_CREDENTIALS_ACCOUNT --key-file <(printf "%%s" "$GOOGLE_CREDENTIALS");
printf 'Run the following command to revoke access credentials:\n$ eval $(gardenctl provider-env --garden garden --project project --shoot shoot -u %[1]s)\n';

# Run this command to configure gcloud for your shell:
# eval $(gardenctl provider-env %[1]s)
`

		const unsetFormat = `gcloud auth revoke $GOOGLE_CREDENTIALS_ACCOUNT --verbosity=error;
unset GOOGLE_CREDENTIALS;
unset GOOGLE_CREDENTIALS_ACCOUNT;
unset CLOUDSDK_CORE_PROJECT;
unset CLOUDSDK_COMPUTE_REGION;
unset CLOUDSDK_CONFIG;

# Run this command to reset the gcloud configuration for your shell:
# eval $(gardenctl provider-env -u %[1]s)
`
		var (
			clientEmail = "john.doe@example.org"
			projectID   = "test"
			configDir   = "config-dir"
		)

		BeforeEach(func() {
			providerType = "gcp"
			commandPath = "gardenctl provider-env"
			filenames = append(filenames, "usage-hint", providerType)
		})

		JustBeforeEach(func() {
			data["credentials"] = map[string]interface{}{
				"client_email": clientEmail,
				"project_id":   projectID,
			}
			data["configDir"] = configDir
		})

		DescribeTable("executing the bash template",
			func(unset bool, format string) {
				metadata["shell"] = shell
				metadata["unset"] = unset
				Expect(t.ExecuteTemplate(out, shell, data)).To(Succeed())
				Expect(out.String()).To(Equal(fmt.Sprintf(format, shell, region, clientEmail, projectID, configDir)))
			},
			Entry("export environment variables", false, exportFormat),
			Entry("unset environment variables", true, unsetFormat),
		)
	})

	Describe("parsing the azure template", func() {
		const exportFormat = `$Env:AZURE_CLIENT_ID = '%[2]s';
$Env:AZURE_CLIENT_SECRET = '%[3]s';
$Env:AZURE_TENANT_ID = '%[4]s';
$Env:AZURE_SUBSCRIPTION_ID = '%[5]s';
$Env:AZURE_CONFIG_DIR = '%[6]s';
az login --service-principal --username "$Env:AZURE_CLIENT_ID" --password "$Env:AZURE_CLIENT_SECRET" --tenant "$Env:AZURE_TENANT_ID";
az account set --subscription "$Env:AZURE_SUBSCRIPTION_ID";
printf 'Run the following command to log out and remove access to Azure subscriptions:\n$ & gardenctl provider-env --garden garden --project project --shoot shoot -u %[1]s | Invoke-Expression\n';
# Run this command to configure az for your shell:
# & gardenctl provider-env %[1]s | Invoke-Expression
`

		const unsetFormat = `az logout --username "$Env:AZURE_CLIENT_ID";
Remove-Item -ErrorAction SilentlyContinue Env:\AZURE_CLIENT_ID;
Remove-Item -ErrorAction SilentlyContinue Env:\AZURE_CLIENT_SECRET;
Remove-Item -ErrorAction SilentlyContinue Env:\AZURE_TENANT_ID;
Remove-Item -ErrorAction SilentlyContinue Env:\AZURE_SUBSCRIPTION_ID;
Remove-Item -ErrorAction SilentlyContinue Env:\AZURE_CONFIG_DIR;
# Run this command to reset the az configuration for your shell:
# & gardenctl provider-env -u %[1]s | Invoke-Expression
`
		var (
			clientID       = "client"
			clientSecret   = "secret"
			tenantID       = "tenant"
			subscriptionID = "subscription"
			configDir      = "config-dir"
		)

		BeforeEach(func() {
			providerType = "azure"
			shell = "powershell"
			commandPath = "gardenctl provider-env"
			filenames = append(filenames, "usage-hint", providerType)
		})

		JustBeforeEach(func() {
			data["clientID"] = clientID
			data["clientSecret"] = clientSecret
			data["tenantID"] = tenantID
			data["subscriptionID"] = subscriptionID
			data["configDir"] = configDir
		})

		DescribeTable("executing the bash template",
			func(unset bool, format string) {
				metadata["shell"] = shell
				metadata["unset"] = unset
				Expect(t.ExecuteTemplate(out, shell, data)).To(Succeed())
				Expect(out.String()).To(Equal(fmt.Sprintf(format, shell, clientID, clientSecret, tenantID, subscriptionID, configDir)))
			},
			Entry("export environment variables", false, exportFormat),
			Entry("unset environment variables", true, unsetFormat),
		)
	})

	Describe("parsing custom templates", func() {
		var filename string

		BeforeEach(func() {
			filenames = append(filenames, "usage-hint")
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
			const exportFormat = `export TEST_TOKEN='%[2]s';

# Run this command to configure test for your shell:
# eval $(gardenctl provider-env %[1]s)
`
			var token string

			BeforeEach(func() {
				providerType = "test"
				token = "token"
				filename = filepath.Join("templates", providerType+".tmpl")
				writeTempFile(filename, readTestFile("templates/"+providerType+".tmpl"))
				data["testToken"] = token
			})

			It("should successfully parse the template", func() {
				Expect(t.ParseFiles(filepath.Join(gardenHomeDir, filename))).To(Succeed())
				Expect(t.ExecuteTemplate(out, shell, data)).To(Succeed())
				Expect(out.String()).To(Equal(fmt.Sprintf(exportFormat, shell, token)))
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
