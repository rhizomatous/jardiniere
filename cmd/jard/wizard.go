package main

import (
	"context"
	"fmt"
	"os"

	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"
	"github.com/spf13/cobra"

	"github.com/rhizomatous/jardiniere/internal/api"
	"github.com/rhizomatous/jardiniere/internal/proxy"
	"github.com/rhizomatous/jardiniere/internal/ui"
)

// choosePolicy asks which posture to start from, the first time anything needs
// a policy, and stores the answer.
//
// Without a terminal it takes the balanced default rather than failing. A
// script that runs `jard run` should not stop on a question nobody is there to
// answer, and balanced is the option the prompt recommends anyway.
func choosePolicy(ctx context.Context, cmd *cobra.Command, svc api.Service) (proxy.Policy, error) {
	preset := proxy.PresetBalanced

	if isTerminal(os.Stdin) {
		var err error
		if preset, err = askPreset(cmd); err != nil {
			return proxy.Policy{}, err
		}
	} else {
		_, _ = lipgloss.Fprintln(cmd.ErrOrStderr(), ui.Faint.Render(
			"no network policy set; starting from the balanced preset. `jard policy` changes it."))
	}

	p := proxy.New(preset)
	if err := svc.SetPolicy(ctx, p); err != nil {
		return proxy.Policy{}, err
	}
	return p, nil
}

// askPreset runs the chooser.
func askPreset(cmd *cobra.Command) (proxy.Preset, error) {
	options := make([]huh.Option[proxy.Preset], 0, len(proxy.Presets))
	for _, p := range proxy.Presets {
		options = append(options, huh.NewOption(string(p)+" — "+p.Description(), p))
	}

	choice := proxy.PresetBalanced
	form := huh.NewForm(huh.NewGroup(
		huh.NewSelect[proxy.Preset]().
			Title("What should sandboxes be allowed to reach?").
			Description("A sandbox has no way out except through jard's proxy.\n" +
				"This can be changed at any time with `jard policy`.").
			Options(options...).
			Value(&choice),
	))

	if err := form.Run(); err != nil {
		return "", fmt.Errorf("choosing a network policy: %w", err)
	}
	_, _ = lipgloss.Fprintln(cmd.ErrOrStderr(),
		ui.OK.Render("network policy ")+ui.Value.Render(string(choice)))
	return choice, nil
}
