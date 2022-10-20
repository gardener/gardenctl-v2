/*
SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package info

import (
	"fmt"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/gardener/gardenctl-v2/internal/util"
	"github.com/gardener/gardenctl-v2/pkg/cmd/base"
)

// InfoOptions is a struct to support Info command
type Options struct {
	base.Options
	// Allocatable is the seed allocatable
	Allocatable int64
	// Capacity is the seed capacity
	Capacity int64
}

// NewInfoOptions returns initialized Options
func NewInfoOptions(ioStreams util.IOStreams) *Options {
	return &Options{
		Options: base.Options{
			IOStreams: ioStreams,
		},
	}
}

// Run does the actual work of the command
func (o *Options) Run(f util.Factory) error {
	manager, err := f.Manager()
	if err != nil {
		return err
	}

	infoTarget, err := manager.CurrentTarget()
	if err != nil {
		return err
	}

	// create client for the garden cluster
	gardenClient, err := manager.GardenClient(infoTarget.GardenName())
	if err != nil {
		return err
	}

	shootList, err := gardenClient.ListShoots(f.Context(), infoTarget.WithShootName("").AsListOption())
	if err != nil {
		return err
	}

	seedList, err := gardenClient.ListSeeds(f.Context(), infoTarget.WithSeedName("").AsListOption())
	if err != nil {
		return err
	}

	var (
		unscheduled                  = 0
		hibernatedShootsCount        = 0
		totalShootsCountPerSeed      = make(map[string]int)
		hibernatedShootsCountPerSeed = make(map[string]int)
		unscheduledList              = make([]string, 0)
		infoOptions                  = make(map[string]Options)
		valAllocatable               int64
		valCapacity                  int64
	)

	for _, seed := range seedList.Items {
		allocatable := seed.Status.Allocatable["shoots"]
		capacity := seed.Status.Capacity["shoots"]

		if v, ok := allocatable.AsInt64(); ok {
			valAllocatable = v
		} else {
			return fmt.Errorf("allocatable conversion is not possible")
		}

		if v, ok := capacity.AsInt64(); ok {
			valCapacity = v
		} else {
			return fmt.Errorf("capacity conversion is not possible")
		}

		infoOptions[seed.Name] = Options{Allocatable: valAllocatable, Capacity: valCapacity}
	}

	for _, shoot := range shootList.Items {
		if shoot.Spec.SeedName == nil {
			// unscheduledList usually list pending clusters during creation
			unscheduledList = append(unscheduledList, shoot.Name)
			unscheduled++

			continue
		}
		totalShootsCountPerSeed[*shoot.Spec.SeedName]++

		if shoot.Status.IsHibernated {
			hibernatedShootsCountPerSeed[*shoot.Spec.SeedName]++
			hibernatedShootsCount++
		}
	}

	var sortedSeeds []string
	for seed := range totalShootsCountPerSeed {
		sortedSeeds = append(sortedSeeds, seed)
	}

	sort.Strings(sortedSeeds)
	fmt.Fprintf(o.IOStreams.Out, "Garden: %s\n", infoTarget.GardenName())

	w := tabwriter.NewWriter(o.IOStreams.Out, 6, 0, 20, ' ', 0)
	fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n", "Seed", "Total", "Active", "Hibernated", "Allocatable", "Capacity")
	fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n", "----", "-----", "------", "----------", "-----------", "--------")

	for _, seed := range sortedSeeds {
		if v, found := infoOptions[seed]; found {
			fmt.Fprintf(w, "%s\t%d\t%d\t%d\t%d\t%d\n", seed, totalShootsCountPerSeed[seed], totalShootsCountPerSeed[seed]-hibernatedShootsCountPerSeed[seed], hibernatedShootsCountPerSeed[seed], v.Allocatable, v.Capacity)
		}
	}

	fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n", "----", "-----", "------", "----------", "-----------", "--------")
	fmt.Fprintf(w, "%s\t%d\t%d\t%d\t%s\t%s\n", "TOTAL", len(shootList.Items), len(shootList.Items)-hibernatedShootsCount-unscheduled, hibernatedShootsCount, "-", "-")
	fmt.Fprintf(w, "%s\t%d\n", "Unscheduled", unscheduled)
	fmt.Fprintf(w, "%s\t%s\n", "Unscheduled List", strings.Join(unscheduledList, ","))
	fmt.Fprintln(w)
	w.Flush()

	return nil
}
