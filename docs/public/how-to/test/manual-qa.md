# Manual QA Infrastructure

The E2E framework can provision real cloud infrastructure for manual QA without
running automated tests. Environments stay alive until you explicitly destroy
them, giving you direct access (SSH, kubectl, RDP) to inspect the agent in a
realistic environment.

## Prerequisites

Complete the [one-time setup](e2e.md#one-time-setup) from the E2E testing guide
before creating any environment.

## Stack lifecycle

Each environment is a named Pulumi stack. Stacks are automatically prefixed
with your OS username:

```
<username>-<stack-name>
# e.g.  alice-aws-vm    (default for the aws/vm scenario)
#       alice-my-qa     (with --stack-name my-qa)
```

Stacks **persist until you destroy them**. Run the matching `destroy-*` task
when you are done to avoid leaving cloud resources running.

Agents are automatically tagged `stackid:<stack-name>` so you can filter
metrics and logs in the Datadog UI to a specific environment.

## Scenarios

| Scenario | Command | What it creates |
|----------|---------|-----------------|
| [AWS VM](manual-qa/aws-vm.md) | `dda inv aws.create-vm` | EC2 instance + Agent |
| [AWS Docker VM](manual-qa/aws-docker.md) | `dda inv aws.create-docker` | EC2 + Docker + containerized Agent |
| [AWS EKS](manual-qa/aws-eks.md) | `dda inv aws.create-eks` | EKS cluster + Agent DaemonSet |
| [AWS ECS](manual-qa/aws-ecs.md) | `dda inv aws.create-ecs` | ECS cluster + Agent |
| [AWS KinD](manual-qa/aws-kind.md) | `dda inv aws.create-kind` | EC2 + KinD cluster + Agent |
| [Azure VM](manual-qa/azure-vm.md) | `dda inv az.create-vm` | Azure VM + Agent |
| [GCP VM](manual-qa/gcp-vm.md) | `dda inv gcp.create-vm` | GCP VM + Agent |

## FakeIntake

All scenarios support `--use-fakeintake`, which deploys a mock Datadog intake
alongside the agent. Payloads are captured locally so you can inspect them
without sending data to a production org. Dual-shipping is enabled by default,
so the agent also sends to the real Datadog backend.

## See Also

- [Running E2E tests](e2e.md) — automated test execution against the same infrastructure
- [test/e2e-framework](../../../../test/e2e-framework/AGENTS.md) — framework internals and provisioner API
