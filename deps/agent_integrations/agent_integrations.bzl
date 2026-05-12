"""Repository rule to retrieve and install integrations and their dependencies."""

def _agent_integrations_impl(ctx):
    # TODO: retrieve git repository at the expected reference.

    # TODO: Parse lockfile


agent_integrations = repository_rule(
    implementation = _agent_integrations_impl,
    attrs = {
        "url": attr.string(
            default="https://github.com/DataDog/integrations-core.git",
            doc="URL for the repository",
        )
        "git_reference": attr.string(mandatory=True),  # TODO: Extract from json / env
        "deps_lockfile_name": attr.string(
            mandatory=True,
            doc="Name of the lockfile to use for fetching dependencies (platform-dependent)",
        ),
    },
    doc =
    """Retrieves the integrations repository and provides rules to install the integrations
    and their dependencies in a Python environment.""",
)
