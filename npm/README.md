# lynxor

A 10-second security sanity check for Git repositories: committed secrets, exposed keys, risky Dockerfile patterns, CI/CD misconfigurations, and known-vulnerable dependencies.

This package is a thin installer: `postinstall` downloads the precompiled `lynxor` binary matching your platform from [GitHub Releases](https://github.com/xchebila/lynxor/releases), verifies its checksum, and wires it up as the `lynxor` command. Supported: Linux/macOS/Windows on amd64/arm64.

```bash
npm install -g lynxor
lynxor scan .
```

Full documentation, flags, and design decisions: [github.com/xchebila/lynxor](https://github.com/xchebila/lynxor).
