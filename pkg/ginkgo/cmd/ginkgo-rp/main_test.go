package main

import (
	"reflect"
	"testing"
)

func TestBuildGinkgoArgs(t *testing.T) {
	cases := []struct {
		name string
		in   []string
		want []string
	}{
		{"no args", nil, []string{"--skip-package=rpagent"}},
		{"flags only", []string{"-r", "-p"}, []string{"-r", "-p", "--skip-package=rpagent"}},
		{
			"flags after paths move before them",
			[]string{"-r", "calculator/", "rp-features/", "--keep-going"},
			[]string{"-r", "--keep-going", "--skip-package=rpagent", "calculator/", "rp-features/"},
		},
		{
			"value flag stays paired",
			[]string{"--procs", "4", "./pkg/auth"},
			[]string{"--procs", "4", "--skip-package=rpagent", "./pkg/auth"},
		},
		{
			"merge into existing",
			[]string{"--skip-package=foo"},
			[]string{"--skip-package=foo,rpagent"},
		},
		{
			"merge space form",
			[]string{"--skip-package", "foo"},
			[]string{"--skip-package=foo,rpagent"},
		},
		{
			"already present is untouched",
			[]string{"--skip-package=foo,rpagent", "./..."},
			[]string{"--skip-package=foo,rpagent", "./..."},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := buildGinkgoArgs(c.in); !reflect.DeepEqual(got, c.want) {
				t.Errorf("got %v, want %v", got, c.want)
			}
		})
	}
}

func TestShouldFinalizeLaunch(t *testing.T) {
	cases := map[string]struct {
		args []string
		want bool
	}{
		"no args":         {nil, true},
		"normal run":      {[]string{"-r", "--keep-going"}, true},
		"dry-run":         {[]string{"--dry-run"}, false},
		"dry-run=true":    {[]string{"--dry-run=true"}, false},
		"dry-run=false":   {[]string{"--dry-run=false"}, true},
		"build-only":      {[]string{"--build-only"}, false},
		"labels flag":     {[]string{"--labels"}, false},
		"help flag":       {[]string{"-r", "--help"}, false},
		"utility command": {[]string{"outline"}, false},
	}
	for name, c := range cases {
		if got := shouldFinalizeLaunch(c.args); got != c.want {
			t.Errorf("%s: got %v, want %v", name, got, c.want)
		}
	}
}

func TestIsPackagePath(t *testing.T) {
	cases := map[string]bool{
		"./...": true, "...": true, "./pkg/auth": true, "../x": true, "calculator/": true,
		"--keep-going": false, "-r": false, "calculator": false,
	}
	for in, want := range cases {
		if got := isPackagePath(in); got != want {
			t.Errorf("isPackagePath(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestDetectTypo(t *testing.T) {
	if ok, hint := detectTypo("--h"); !ok || hint == "" {
		t.Errorf("--h should be flagged with a suggestion")
	}
	if ok, _ := detectTypo("--focus"); ok {
		t.Errorf("--focus is a real ginkgo flag and must pass through")
	}
}
