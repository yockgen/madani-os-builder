# Image Composer Tool

[![License](https://img.shields.io/badge/License-MIT-blue.svg)](./LICENSE)

The Image Composer Tool (ICT) is a toolchain that enables building immutable
Linux distributions using a simple toolchain from pre-built packages emanating
from different Operating System Vendors (OSVs).

The ICT is developed in `golang` and is initially targeting to build custom
images for [EMT](https://github.com/open-edge-platform/edge-microvisor-toolkit)
(Edge Microvisor Toolkit), [Azure Linux](https://github.com/microsoft/azurelinux)
and WindRiver Linux.

## Get Started

### Building

Build the image-composer

```bash
go build ./cmd/image-composer
```

Run `go test` to validate a given JSON file against the schema

```bash
go test ./internal/validate
```

Run the image composer with the AzureLinux provider. The testdata/valid.json file
is composed to use AzureLinux v3. The provider name is created by `name`+`version`,
so the provider name is `azurelinux3`.

```bash
go run ./cmd/image-composer testdata/valid.json
```

With Earthfile, simply run:

```bash
earthly +build
```

### User Input JSON

This section provides a detailed explanation of the JSON input structure used to configure and build a Linux-based operating system. The JSON format allows users to define key parameters such as the operating system distribution, version, architecture, software packages, output format, immutability, and kernel configuration. By customizing these parameters, users can create tailored Linux OS builds, including distributions like Ubuntu, Wind River, and Edge Microvisor Toolkits.

```json
{
    "distro": "eLxr",
    "version": "12",
    "arch": "x86_64",
    "packages": [
        "cloud-init",        
        "cloud-utils-growpart",
        "dhcpcd",        
        "grubby",
        "hyperv-daemons",
        "netplan",        
        "python3",
        "rsyslog",
        "sgx-backwards-compatibility",
        "WALinuxAgent",        
        "wireless-regdb"        
    ],
    "immutable": true,
    "output": "iso",
    "kernel": {
      "version": "5.10.0",
      "cmdline": "quiet splash"
    }
}
```

#### Key Components

##### 1. `distro`

**Description:** Specifies the target Linux distribution to be built.  
**Examples:**

- `AzureLinux`
- `eLxr`

##### 2. `version`

**Description:** Defines the version of the target operating system.  
**Example:** `"12"`

##### 3. `arch`

**Description:** Specifies the architecture of the target operating system.  
**Examples:**

- `x86_64`
- `arm64`

##### 4. `packages`

**Description:** A list of software packages to be included in the OS build. These packages will be pre-installed in the resulting image.  
**Examples:**

- `cloud-init`: Used for initializing cloud instances.
- `python3`: The Python 3 programming language interpreter.
- `rsyslog`: A logging system for Linux.

##### 5. `immutable`

**Description:** Indicates whether the operating system should be immutable.  
**Values:**

- `true`: The OS is immutable, meaning it cannot be modified after creation.
- `false`: The OS is mutable, allowing modifications after creation.

##### 6. `output`

**Description:** Specifies the format of the output build.  
**Values:**

- `iso`: The OS will be built as an ISO file, suitable for installation or booting.
- `raw`: The OS will be built as a raw disk image, useful for direct disk writing.
- `vhd`: The OS will be built as a VHD (Virtual Hard Disk) file, commonly used in virtual environments.

##### 7. `kernel`

**Description:** Defines the kernel version and allows customization of the kernel command line.  
**Attributes:**

- `version`: Specifies the kernel version to be used.  
  **Example:** `"5.10.0"`
- `cmdline`: Provides additional kernel command-line parameters.  
  **Example:** `"quiet splash"`

Run the sample JSON files against the defined [schema](schema/os-image-composer.schema.json).
There are two sample JSON files, one [valid](/testdata/valid.json) and one with
[invalid](testdata/invalid.json) content.

## Getting Help

## Contributing

## License Information
