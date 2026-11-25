/*
SPDX-FileCopyrightText: Contributors to the Gardener project

SPDX-License-Identifier: Apache-2.0
*/

package config_test

import (
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/gardener/gardenctl-v2/internal/fake"
	cmdconfig "github.com/gardener/gardenctl-v2/pkg/cmd/config"
	"github.com/gardener/gardenctl-v2/pkg/config"
)

var _ = Describe("config add-seed-key", func() {
	var keyFile string

	BeforeEach(func() {
		dir := GinkgoT().TempDir()
		keyFile = filepath.Join(dir, "public-key.pem")
		cfg.Filename = filepath.Join(dir, "config.yaml")
		cfg.Gardens[0].Alias = "dev"
		Expect(cfg.Save()).To(Succeed())
	})

	run := func(garden string, flags ...string) error {
		GinkgoHelper()
		factory.EXPECT().Manager().Return(manager, nil)
		manager.EXPECT().Configuration().Return(cfg)

		cmd := cmdconfig.NewCmdConfig(factory, streams)
		cmd.SetArgs(append([]string{"add-seed-key", garden}, flags...))

		return cmd.Execute()
	}

	It("adds keys from files and inline PEM to the named Garden and skips duplicates", func() {
		_, first, err := fake.NewSignedSeedKubeconfigSecret("seed-a")
		Expect(err).NotTo(HaveOccurred())
		_, second, err := fake.NewSignedSeedKubeconfigSecret("seed-a")
		Expect(err).NotTo(HaveOccurred())

		Expect(os.WriteFile(keyFile, []byte(first[0]), 0o600)).To(Succeed())
		Expect(run("dev", "--file", keyFile)).To(Succeed())
		Expect(run(gardenIdentity1, "--key", strings.TrimSuffix(second[0], "\n"))).To(Succeed())

		sameKey := strings.ReplaceAll(first[0], "\n", "\r\n")
		Expect(run("dev", "--key", sameKey)).To(Succeed())
		Expect(out.String()).To(ContainSubstring("already trusted"))

		Expect(cfg.Gardens[0].SeedKubeconfigPublicKeys).To(Equal(config.SeedKubeconfigPublicKeys{first[0], second[0]}))
		Expect(cfg.Gardens[1].SeedKubeconfigPublicKeys).To(BeEmpty())
		assertConfigHasBeenSaved(cfg)
	})

	It("preserves the configuration on errors", func() {
		original, err := os.ReadFile(cfg.Filename)
		Expect(err).NotTo(HaveOccurred())

		Expect(run("unknown", "--file", keyFile)).To(MatchError(And(
			ContainSubstring(`garden "unknown" is not defined`),
			ContainSubstring(cfg.Filename),
			ContainSubstring("gardenctl config set-garden"),
		)))
		Expect(run("dev", "--file", keyFile)).To(MatchError(ContainSubstring("failed to read public key")))
		Expect(os.WriteFile(keyFile, []byte("invalid key"), 0o600)).To(Succeed())
		Expect(run("dev", "--file", keyFile)).To(MatchError(ContainSubstring("expected a PEM PUBLIC KEY block")))
		Expect(run("dev", "--key", "invalid key")).To(MatchError(ContainSubstring("expected a PEM PUBLIC KEY block")))

		Expect(os.ReadFile(cfg.Filename)).To(Equal(original))
	})

	It("rejects adding a key to a Garden that opted out of signature verification", func() {
		cfg.Gardens[0].InsecureSkipSeedKubeconfigSignatureVerification = true
		Expect(cfg.Save()).To(Succeed())

		_, keys, err := fake.NewSignedSeedKubeconfigSecret("seed-a")
		Expect(err).NotTo(HaveOccurred())
		Expect(run("dev", "--key", keys[0])).To(MatchError(ContainSubstring("Remove insecureSkipSeedKubeconfigSignatureVerification before adding a key")))

		Expect(cfg.Gardens[0].SeedKubeconfigPublicKeys).To(BeEmpty())
		assertConfigHasBeenSaved(cfg)
	})

	It("shows help without loading the configuration", func() {
		cmd := cmdconfig.NewCmdConfig(factory, streams)
		cmd.SetOut(out)
		cmd.SetArgs([]string{"add-seed-key", "--help"})

		Expect(cmd.Execute()).To(Succeed())
		Expect(out.String()).To(ContainSubstring("add-seed-key GARDEN (--file PUBLIC_KEY.pem | --key PEM)"))
	})

	It("points to help when the Garden argument is missing", func() {
		cmd := cmdconfig.NewCmdConfig(factory, streams)
		cmd.SetArgs([]string{"add-seed-key", "--file", keyFile})

		Expect(cmd.Execute()).To(MatchError(ContainSubstring("provide one Garden identity or alias. See 'config add-seed-key --help'")))
	})

	DescribeTable("requires exactly one key source", func(flags []string, message string) {
		cmd := cmdconfig.NewCmdConfig(factory, streams)
		cmd.SetArgs(append([]string{"add-seed-key", "dev"}, flags...))

		Expect(cmd.Execute()).To(MatchError(ContainSubstring(message)))
	},
		Entry("neither flag", nil, "at least one"),
		Entry("both flags", []string{"--file=key.pem", "--key=pem"}, "none of the others can be"),
	)
})
