"""Repository rule to retrieve and install integrations and their dependencies."""

def _detect_platform(ctx):
    """Returns a (os, arch) tuple normalized to our lockfile naming convention."""
    os_name = ctx.os.name
    arch = ctx.os.arch

    if "linux" in os_name:
        os = "linux"
    elif "mac" in os_name:
        os = "macos"
    elif "windows" in os_name:
        os = "windows"
    else:
        fail("Unsupported OS: " + os_name)

    if arch in ("x86_64", "amd64"):
        arch = "x86_64"
    elif arch in ("aarch64", "arm64"):
        arch = "aarch64"
    else:
        fail("Unsupported architecture: " + arch)

    return (os, arch)

def _parse_lockfile(content, wheels_storage):
    """Parse a PEP 440 direct-reference lockfile.

    Each non-blank, non-comment line has the form:
        package-name @ URL#sha256=HASH

    ${INTEGRATIONS_WHEELS_STORAGE} placeholders in URLs are substituted with wheels_storage.
    Returns a list of structs with fields: name, url, sha256, filename.
    """
    wheels = []
    for line in content.splitlines():
        line = line.strip()
        if not line or line.startswith("#"):
            continue

        parts = line.split(" @ ", 1)
        if len(parts) != 2:
            fail("Unexpected lockfile line: " + line)

        name = parts[0].strip()
        url_with_hash = parts[1].strip().replace("${INTEGRATIONS_WHEELS_STORAGE}", wheels_storage)

        url_parts = url_with_hash.split("#sha256=", 1)
        if len(url_parts) != 2:
            fail("Missing #sha256= fragment in lockfile line: " + line)

        url = url_parts[0]
        sha256 = url_parts[1]
        filename = url.split("/")[-1]

        wheels.append(struct(
            name = name,
            url = url,
            sha256 = sha256,
            filename = filename,
        ))

    return wheels

def _agent_integrations_impl(ctx):
    os, arch = _detect_platform(ctx)
    python_version = ctx.attr.python_version
    lockfile_name = "{}-{}_{}.txt".format(os, arch, python_version)

    commit = ctx.attr.commit
    ctx.download_and_extract(
        url = "{base_url}/archive/{commit}.tar.gz".format(
            base_url = ctx.attr.base_url,
            commit = commit,
        ),
        sha256 = ctx.attr.sha256,
        strip_prefix = "integrations-core-{}".format(commit),
    )

    lockfile_content = ctx.read(".deps/resolved/{}".format(lockfile_name))
    wheels = _parse_lockfile(lockfile_content, ctx.attr.wheels_storage)

    for wheel in wheels:
        ctx.download(
            url = wheel.url,
            output = "wheelhouse/{}".format(wheel.filename),
            sha256 = wheel.sha256,
        )

    # TODO: Generate the build file such that it will have a rule that
    # gets a python executable or environment as an input and pip installs
    # the downloaded packages
    ctx.file("BUILD.bazel", "")

agent_integrations = repository_rule(
    implementation = _agent_integrations_impl,
    attrs = {
        "base_url": attr.string(
            default = "https://github.com/DataDog/integrations-core",
            doc = "Base URL of the repository",
        ),
        "commit": attr.string(mandatory = True),
        "sha256": attr.string(),
        "wheels_storage": attr.string(
            mandatory = True,
            doc = "Value substituted for ${INTEGRATIONS_WHEELS_STORAGE} in wheel URLs",
        ),
        "python_version": attr.string(
            default = "3.13",
            doc = "Python version string used to select the platform lockfile",
        ),
    },
    doc =
        """Retrieves the integrations repository and provides rules to install the integrations
    and their dependencies in a Python environment.""",
)
