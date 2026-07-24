# reposcan

A 10-second security sanity check for Git repositories: committed secrets, exposed keys, risky Dockerfile patterns, CI/CD misconfigurations, and known-vulnerable dependencies.

This package is a thin installer: `postinstall` downloads the precompiled `reposcan` binary matching your platform from [GitHub Releases](https://github.com/xchebila/reposcan/releases), verifies its checksum, and wires it up as the `reposcan` command. Supported: Linux/macOS/Windows on amd64/arm64.

```bash
npm install -g reposcan
reposcan scan .
```

Full documentation, flags, and design decisions: [github.com/xchebila/reposcan](https://github.com/xchebila/reposcan).
