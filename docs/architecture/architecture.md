# OS Image Composer Architecture

OS Image Composer is a generic toolkit to build operating system
images from pre-built artifacts such as `rpm` and `deb` in order to support
a range of common operating systems for the distributed edge. You can customize
the content of the operating system to suit your requirements, applications,
and workloads.

## Table of Contents

- [Supported Distributions](#supported-distributions)
- [Overview](#overview)
- [OS Image Composer System Network Context](#os-image-composer-system-network-context)
  - [Network Security Considerations](#network-security-considerations)
  - [Package Sign Verification](#package-sign-verification)
- [Components Overview](#components-overview)
  - [Chroot](#chroot)
  - [Image](#image)
  - [Config](#config)
  - [OSPackage](#ospackage)
- [Operational Flow](#operational-flow)
- [Related Documentation](#related-documentation)

## Supported Distributions

The OS Image Composer tool will initially support the following
Linux distributions:

- [Edge Microvisor Toolkit](https://github.com/open-edge-platform/edge-microvisor-toolkit)
  3.0
- [Azure Linux](https://github.com/microsoft/azurelinux) 3.0
- [Wind River eLxr](https://www.windriver.com/blog/Introducing-eLxr)

## Overview

The OS Image Composer tool generates an image based on a user-customized YAML
input file that defines the customizations for a supported OS distribution.
Multiple operating systems are supported by the corresponding configuration
files. And a set of default image config files per OS distribution helps you
get rid of being trapped in the details of OS system configurations. The user
input YAML file can be simply nothing configured or full detailed
configurations. During the image generation, the final config JSON file is
based on the default JSON config and updated according to the user-customized
YAML file.

The pre-built artifacts such as those for `rpm` and `deb` come from their
corresponding target operating system's remote repository. The OS Image
Composer tool implemented the common abstracted package provider interface to
fetch packages and solve dependencies from those remote package repositories.
To download packages, the OS Image Composer tool securely fetches packages
from the distribution-specific package repository and automatically resolves
and installs dependencies.

The runtime storage includes the isolated chroot build environment (the rootfs
entry for the target operating system), which is the image-generation workspace,
the package caches, and the image caches as well as the runtime configuration
files and logs.

The supported output image types include raw images and ISO images for bare
metal systems and virtual machines.

The following diagram shows the input and output of the OS Image Composer tool:

![Overview](assets/overview.drawio.svg)

## OS Image Composer System Network Context

The following diagram shows the network context of the OS Image Composer tool:

![OS Image Composer Network Diagram](assets/os-image-composer-network-diagram.drawio.svg).

The diagram illustrates how different components of the product's system
architecture communicate with each other as well as the flow of setup and
deployment process:

1. Clone image composer tool repository from Intel Open Edge Platform
2. Install or Build the Image Composer Tool to the target architecture.
3. Use the Image Composer Tool to compose your custom OS Image for the desired
   OS distribution. The remote package repositories perform server side
   validation on the HTTPS requests from the image composer tool. Integrity of
   each downloaded package is verified using authenticated GPG keys.
4. Deploy the composed OS Image on edge node using your preferred
   deployment method.

### Network Security Considerations

The OS Image Composer tool downloads required packages using HTTP requests to
the distribution specific package repos over TLS 1.2+ connections. Each of the
package repos does server-side validation on the package download requests so
it is expected that the system running the OS Image Composer tool is provisioned
with a CA root chain.

### Package Sign Verification

When packages are downloaded, they are verified for integrity by using the GPG
Public Keys and SHA256/MD5 checksum published at the package repositories.

## Components Overview

The following diagram outlines the high-level components of
the OS Image Composer tool:

![components high level view](assets/components.drawio.svg)

The tools for composing an image are grouped under following components:
**Provider**, **Chroot**, **Image**, **OsPackage**, and **Config**.
For modularity, each group contains a set of the components for the
OS Image Composer tool's functions.

The **provider** component takes data from *config* as its input, then calls
**chroot**, **image** and **OsPackage** components to set up building the image.
**Chroot** libraries are used to create ChrootEnv for building the OS Image.
The **image** libraries provide the general functions for building OS images.
The **OsPackage** libraries include utilities for handling debian and rpm packages.
The **config** component contains configuration data for the image that will be
created.

### Chroot

The OS Image Composer tool generates a `chroot` environment, which is used for
the image composition and creation process which is isolated from the host
operating file system. Packages are fetched, with help from a provider, and
cached locally before the image is created.

### Image

![components - image](assets/components.drawio.image.svg)

*Image* groups the libraries that generate the image; they are divided according
to the generic processing flow. It creates the required raw or ISO images
according to an image configuration JSON file.

### Config

![components - Config](assets/components.drawio.config.svg)

The *config* component contains configuration data for the image that will be
created. It serves as input data for the *Provider*, which builds the OS image
according to that data.

<!--The host system should be running a validated and supported Linux distribution
such as Ubuntu 24.04. The OS Image Composer tool implements all the common business
logic, which remains the same regardless of a target OS distribution.
Providers exists for the supported operating systems with an
interface definition that each provider needs to implement to decouple
distribution-specific functionality from the core and common business logic.-->

### OsPackage

![components - package](assets/components.drawio.OsPackage.svg)

*OsPackage* groups the libraries that provide the unified interface of the
operating system vendors' remote package repositories. It analyzes given
package lists and downloads all the packages and dependencies from the target
operating system's remote package repository to a local cache.

It also verifies signatures of the downloaded packages to ensure they are
authenticated and from a certified source. It also provides the unified
interface to install the packages and the dependencies in the correct order
into the image rootfs directory.

## Operational Flow

The following diagram illustrates the overall image composition flow.

![Image composition workflow](./assets/image.composition.workflow.drawio.svg)

## Related Documentation

- [Understanding the Build Process](./os-image-composer-build-process.md)
- [Understanding Templates](./os-image-composer-templates.md)
- [Multiple Package Repository Support](./os-image-composer-multi-repo-support.md)
- [OS Image Composer CLI Reference](./os-image-composer-cli-specification.md)

<!--hide_directive
:::{toctree}
:hidden:

CLI Specification <os-image-composer-cli-specification>
Security Objectives <image-composition-tool-security-objectives>
Build Process <os-image-composer-build-process>
image-manifest-specification
Coding Style Guide <os-image-composer-coding-style>

:::
hide_directive-->