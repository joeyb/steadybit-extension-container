// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package runc

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/extension-container/config"
	"github.com/steadybit/extension-container/pkg/container/types"
	"github.com/steadybit/extension-container/pkg/utils"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

type Runc struct {
	Root          string
	Debug         bool
	SystemdCgroup bool
	Rootless      string
}

type Container struct {
	ID          string            `json:"id"`
	Pid         int               `json:"pid"`
	Status      string            `json:"status"`
	Bundle      string            `json:"bundle"`
	Rootfs      string            `json:"rootfs"`
	Created     time.Time         `json:"created"`
	Annotations map[string]string `json:"annotations"`
}

func NewRunc(runtime types.Runtime) *Runc {
	root := config.Config.RuncRoot
	if root == "" {
		root = runtime.DefaultRuncRoot()
	}

	return &Runc{
		SystemdCgroup: config.Config.RuncSystemdCgroup,
		Rootless:      config.Config.RuncRootless,
		Root:          root,
		Debug:         config.Config.RuncDebug,
	}
}

func (r *Runc) State(ctx context.Context, id string) (*Container, error) {
	output, err := r.command(ctx, "state", id).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("%s: %s", err, output)
	}

	log.Trace().Str("output", string(output)).Msg("get container state")

	var c Container
	if err := json.Unmarshal(output, &c); err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *Runc) Spec(ctx context.Context, bundle string) error {
	log.Trace().Str("bundle", bundle).Msg("creating container spec")
	output, err := r.command(ctx, "spec", "--bundle", bundle).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %s", err, output)
	}
	return nil
}

type IoOpts struct {
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

func InheritStdIo() IoOpts {
	return IoOpts{
		Stdin:  nil,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}
}

func (o IoOpts) WithStdin(reader io.Reader) IoOpts {
	return IoOpts{
		Stdin:  reader,
		Stdout: o.Stdout,
		Stderr: o.Stderr,
	}
}

func (r *Runc) Run(ctx context.Context, id, bundle string, ioOpts IoOpts) error {
	log.Trace().Str("id", id).Msg("running container")

	cmd := r.command(ctx, "run", "--bundle", bundle, id)
	cmd.Stdin = ioOpts.Stdin
	cmd.Stdout = ioOpts.Stdout
	cmd.Stderr = ioOpts.Stderr
	err := cmd.Run()

	log.Trace().Str("id", id).Int("exitCode", cmd.ProcessState.ExitCode()).Msg("container exited")
	return err
}

func (r *Runc) command(ctx context.Context, args ...string) *exec.Cmd {
	return utils.RootCommandContext(ctx, "runc", append(r.args(), args...)...)
}

func (r *Runc) args() []string {
	out := []string{}
	if r.Root != "" {
		out = append(out, "--root", r.Root)
	}
	if r.Debug {
		out = append(out, "--debug")
	}
	if r.SystemdCgroup {
		out = append(out, "--systemd-cgroup")
	}
	if r.Rootless != "" {
		out = append(out, "--rootless", r.Rootless)
	}
	return out
}

func (r *Runc) Delete(ctx context.Context, id string, force bool) error {
	log.Trace().Str("id", id).Msg("deleting container")
	output, err := r.command(ctx, "delete", fmt.Sprintf("--force=%t", force), id).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %s", err, output)
	}
	log.Trace().Str("id", id).Msg("deleted container")
	return nil
}

func (r *Runc) PrepareBundle(ctx context.Context, image string, id string) (string, func() error, error) {
	bundle := filepath.Join("/tmp/steadybit/containers", id)
	rootfs := filepath.Join(bundle, "rootfs")

	_ = os.RemoveAll(bundle)

	log.Trace().Str("bundle", bundle).Msg("creating container bundle")
	if err := os.MkdirAll(rootfs, 0775); err != nil {
		return "", nil, fmt.Errorf("failed to create bundle dir: %w", err)
	}

	cleanup := func() error {
		log.Trace().Str("bundle", bundle).Msg("cleaning up container bundle")
		return os.RemoveAll(bundle)
	}

	log.Trace().Str("image", image).Str("rootfs", rootfs).Msg("extracting image")
	cmd := exec.Command("tar", "-xf", image, "-C", rootfs)
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", cleanup, fmt.Errorf("failed to prepare rootfs dir: %s %w", out, err)
	}

	if err := r.Spec(ctx, bundle); err != nil {
		return "", cleanup, err
	}

	log.Trace().Str("bundle", bundle).Str("id", id).Msg("prepared container bundle")
	return bundle, cleanup, nil
}
