# Slide SDK
![Slide SDK](https://media.publit.io/file/Landslide/Github/Slide-SDK.png)

LandslideVM is a custom virtual machine designed for AvalancheGo. It enables the execution of Cosmos SDK chains on the Avalanche network by emulating the CometBFT consensus mechanism. This innovative VM allows for the execution of Cosmos SDK chains and interaction with Cosmos modules while being secured by the Avalanche consensus protocol.

## Important Disclaimer

**Landslide is currently in testnet with the release of SlideSDK. Please expect bumps in the road as we continue to develop and refine the system. Use at your own risk and do not use for production environments at this stage.**

## Features

- Execute Cosmos SDK chains on Avalanche
- Emulate CometBFT consensus
- Interact with Cosmos modules
- Secured by Avalanche consensus

## Dependencies

LandslideVM relies on the following major dependencies:

- Go (latest version recommended)
- Cosmos SDK
- AvalancheGo

Please ensure you have the latest versions of these dependencies installed to avoid any known vulnerabilities.

## Installation

[Add installation instructions here]

## Usage

[Add usage instructions here]

## Security

LandslideVM has undergone a security audit by Oak Security GmbH. While efforts have been made to address identified issues, users should exercise caution and understand the following:

- The VM facilitates complex communications with AvalancheGo and implements emulation of CometBFT functionalities.
- Some Cosmos SDK modules that rely on validator information may have limited functionality due to the emulation of the consensus mechanism.
- Node operators should be careful about which gRPC endpoints are exposed publicly.

For a full understanding of the security considerations, please refer to the complete audit report.

## Contributing

[Add contribution guidelines here]

## License

[Add license information here]

## Disclaimer

THIS SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED. USE AT YOUR OWN RISK. AS MENTIONED ABOVE, LANDSLIDE IS CURRENTLY IN TESTNET AND MAY EXPERIENCE INSTABILITY OR UNEXPECTED BEHAVIOR.

For more information about LandslideVM, please [contact us/visit our website].
