# Chocolatey Package for WUT

This directory contains the Chocolatey package configuration for [WUT](https://github.com/thirawat27/wut) - AI-Powered Command Helper.

## Installation

```powershell
# Install from Chocolatey Community Repository
choco install wut
```

## Upgrade

```powershell
choco upgrade wut
```

## Uninstall

```powershell
choco uninstall wut
```

## Package

The Chocolatey package is automatically generated and pushed by [GoReleaser](https://goreleaser.com/) on each release.

## Manual Build (for maintainers)

```powershell
cd chocolatey
choco pack
choco push wut.<version>.nupkg --source https://push.chocolatey.org/
```
