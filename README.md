# Slide SDK
![Slide SDK](https://media.publit.io/file/Landslide/Github/Slide-SDK.png)
[![Contributor Covenant](https://img.shields.io/badge/Contributor%20Covenant-2.1-4baaaa.svg)](CODE_OF_CONDUCT.md)

LandslideVM is a custom virtual machine designed for AvalancheGo. It enables the execution of Cosmos SDK chains on the Avalanche network by emulating the CometBFT consensus mechanism. This innovative VM allows for the execution of Cosmos SDK chains and interaction with Cosmos modules while being secured by the Avalanche consensus protocol.

## Important Disclaimer

**Landslide is currently in testnet with the release of SlideSDK. Please expect bumps in the road as we continue to develop and refine the system. Use at your own risk and do not use for production environments at this stage.**

## Security Audit
The SLIDE SDK has been audited by Oak Security. You can view the full audit report [here](https://github.com/oak-security/audit-reports/blob/main/Slide%20SDK/2024-09-20%20Audit%20Report%20-%20Slide%20SDK%20v1.1.pdf).

## Features

- Execute Cosmos SDK chains on Avalanche
- Emulate CometBFT consensus
- Interact with Cosmos modules
- Secured by Avalanche consensus

## Dependencies

LandslideVM relies on the following major dependencies:

- Go v 1.22.7
- Cosmos SDK 0.50.9 (https://github.com/cosmos/cosmos-sdk/releases/tag/v0.50.9)
- AvalancheGo v1.11.11 (https://github.com/ava-labs/avalanchego/releases/tag/v1.11.11)

Please ensure you have the latest versions of these dependencies installed to avoid any known vulnerabilities.

## Installation

Please follow our docs for installation: (https://docs.landslide.network/)

## Security

LandslideVM has undergone a security audit by Oak Security GmbH. While efforts have been made to address identified issues, users should exercise caution and understand the following:

- The VM facilitates complex communications with AvalancheGo and implements emulation of CometBFT functionalities.
- Some Cosmos SDK modules that rely on validator information may have limited functionality due to the emulation of the consensus mechanism.
- Node operators should be careful about which gRPC endpoints are exposed publicly.

For a full understanding of the security considerations, please refer to the complete audit report.

## Security Audit

Please check [audit](./audit) folder.

## Contributing

We welcome contributions to the Slide SDK! Please see our [CONTRIBUTING.md](CONTRIBUTING.md) file for details on how to contribute.

## License

Slide SDK v0.1 is licensed under the [Business Source License 1.1](./LICENSE). Please see the [LICENSE](./LICENSE) file for details.

## Disclaimer

THIS SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED. USE AT YOUR OWN RISK. AS MENTIONED ABOVE, LANDSLIDE IS CURRENTLY IN TESTNET AND MAY EXPERIENCE INSTABILITY OR UNEXPECTED BEHAVIOR.

For more information about LandslideVM, please [contact us/visit our website].
