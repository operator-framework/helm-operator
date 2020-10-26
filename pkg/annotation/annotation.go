/*
Copyright 2020 The Operator-SDK Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package annotation

import (
	"strconv"

	"helm.sh/helm/v3/pkg/action"

	helmclient "github.com/joelanford/helm-operator/pkg/client"
)

var (
	DefaultInstallAnnotations   = []Install{InstallDescription{}, InstallDisableHooks{}}
	DefaultUpgradeAnnotations   = []Upgrade{UpgradeDescription{}, UpgradeDisableHooks{}, UpgradeForce{}}
	DefaultUninstallAnnotations = []Uninstall{UninstallDescription{}, UninstallDisableHooks{}}
)

type Install interface {
	Name() string
	InstallOption(string) helmclient.InstallOption
}

type Upgrade interface {
	Name() string
	UpgradeOption(string) helmclient.UpgradeOption
}

type Uninstall interface {
	Name() string
	UninstallOption(string) helmclient.UninstallOption
}

type InstallDisableHooks struct {
	CustomName string
}

var _ Install = &InstallDisableHooks{}

const (
	defaultDomain                    = "helm.sdk.operatorframework.io"
	defaultInstallDisableHooksName   = defaultDomain + "/install-disable-hooks"
	defaultUpgradeDisableHooksName   = defaultDomain + "/upgrade-disable-hooks"
	defaultUninstallDisableHooksName = defaultDomain + "/uninstall-disable-hooks"

	defaultUpgradeForceName = defaultDomain + "/upgrade-force"

	defaultInstallDescriptionName   = defaultDomain + "/install-description"
	defaultUpgradeDescriptionName   = defaultDomain + "/upgrade-description"
	defaultUninstallDescriptionName = defaultDomain + "/uninstall-description"
)

func (i InstallDisableHooks) Name() string {
	if i.CustomName != "" {
		return i.CustomName
	}
	return defaultInstallDisableHooksName
}

func (i InstallDisableHooks) InstallOption(val string) helmclient.InstallOption {
	disableHooks := false
	if v, err := strconv.ParseBool(val); err == nil {
		disableHooks = v
	}
	return func(install *action.Install) error {
		install.DisableHooks = disableHooks
		return nil
	}
}

type UpgradeDisableHooks struct {
	CustomName string
}

var _ Upgrade = &UpgradeDisableHooks{}

func (u UpgradeDisableHooks) Name() string {
	if u.CustomName != "" {
		return u.CustomName
	}
	return defaultUpgradeDisableHooksName
}

func (u UpgradeDisableHooks) UpgradeOption(val string) helmclient.UpgradeOption {
	disableHooks := false
	if v, err := strconv.ParseBool(val); err == nil {
		disableHooks = v
	}
	return func(upgrade *action.Upgrade) error {
		upgrade.DisableHooks = disableHooks
		return nil
	}
}

type UpgradeForce struct {
	CustomName string
}

var _ Upgrade = &UpgradeForce{}

func (u UpgradeForce) Name() string {
	if u.CustomName != "" {
		return u.CustomName
	}
	return defaultUpgradeForceName
}

func (u UpgradeForce) UpgradeOption(val string) helmclient.UpgradeOption {
	force := false
	if v, err := strconv.ParseBool(val); err == nil {
		force = v
	}
	return func(upgrade *action.Upgrade) error {
		upgrade.Force = force
		return nil
	}
}

type UninstallDisableHooks struct {
	CustomName string
}

var _ Uninstall = &UninstallDisableHooks{}

func (u UninstallDisableHooks) Name() string {
	if u.CustomName != "" {
		return u.CustomName
	}
	return defaultUninstallDisableHooksName
}

func (u UninstallDisableHooks) UninstallOption(val string) helmclient.UninstallOption {
	disableHooks := false
	if v, err := strconv.ParseBool(val); err == nil {
		disableHooks = v
	}
	return func(uninstall *action.Uninstall) error {
		uninstall.DisableHooks = disableHooks
		return nil
	}
}

var _ Install = &InstallDescription{}

type InstallDescription struct {
	CustomName string
}

func (i InstallDescription) Name() string {
	if i.CustomName != "" {
		return i.CustomName
	}
	return defaultInstallDescriptionName
}
func (i InstallDescription) InstallOption(v string) helmclient.InstallOption {
	return func(i *action.Install) error {
		i.Description = v
		return nil
	}
}

var _ Upgrade = &UpgradeDescription{}

type UpgradeDescription struct {
	CustomName string
}

func (u UpgradeDescription) Name() string {
	if u.CustomName != "" {
		return u.CustomName
	}
	return defaultUpgradeDescriptionName
}
func (u UpgradeDescription) UpgradeOption(v string) helmclient.UpgradeOption {
	return func(upgrade *action.Upgrade) error {
		upgrade.Description = v
		return nil
	}
}

var _ Uninstall = &UninstallDescription{}

type UninstallDescription struct {
	CustomName string
}

func (u UninstallDescription) Name() string {
	if u.CustomName != "" {
		return u.CustomName
	}
	return defaultUninstallDescriptionName
}
func (u UninstallDescription) UninstallOption(v string) helmclient.UninstallOption {
	return func(uninstall *action.Uninstall) error {
		uninstall.Description = v
		return nil
	}
}
