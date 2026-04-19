# Kubexm Test Plan and Test Cases

## 1. Overview
This document outlines the detailed test plan and test cases for the `kubexm` CLI tool. It covers all commands, parameters, execution branches, and deployment matrices defined in the product requirements and architecture plans. The goal is to ensure stability, correctness, and adherence to the architectural design across different deployment scenarios, both in offline and online modes.

## 2. Test Plan

### 2.1 Scope
The testing scope encompasses:
- **CLI Commands**: Verification of all subcommands (`cluster`, `node`, `certs`, `config`, `registry`, `download`, `iso`).
- **Flags & Parameters**: Validation of global flags (`--verbose`, `--yes`) and command-specific flags (e.g., `--config`, `--dry-run`, `--type`).
- **Deployment Matrices**: Covering permutations of `kubernetes_type` (kubeadm, kubexm), `etcd.type` (kubeadm, kubexm, exists), and `loadbalancer` configurations.
- **Architectural Constraints**: Verification of layer separation (Pipeline -> Module -> Task -> Step -> Runner -> Connector), atomicity, idempotency of Steps, and offline/online modes.
- **Error Handling**: Assessing the behavior during invalid inputs and unrecoverable failures.

### 2.2 Test Environment
- Setup local VMs (e.g., VirtualBox, KVM) representing varying topologies (single master, multi-master with worker nodes).
- Dedicated environments mimicking isolated offline networks.

### 2.3 Methodology
- **Unit Testing**: Focus on individual helper functions and parsing utilities.
- **Integration Testing**: Testing Task and Module assemblies, specifically that `Pipeline -> Module -> Task -> Step` execution follows the Directed Acyclic Graph (DAG) correctly.
- **E2E Testing**: Running actual `kubexm` commands against local VM clusters, verifying the final state of the deployed Kubernetes clusters.
- **Idempotency Testing**: Running commands multiple times to ensure no unintended side-effects occur upon subsequent executions.
- **Offline Testing**: Simulating an air-gapped environment using a predefined download base directory.

## 3. Deployment Matrix (Execution Branches)

To ensure comprehensive coverage according to `docs/prd.md` and `REFACTOR_GUIDE.md`, tests will be conducted against the following core matrices:

| Matrix ID | Master Node | `kubernetes_type` | `etcd.type` | `loadbalancer` enable | `loadbalancer_mode` | `loadbalancer_type` | LB Deployment Config |
|-----------|-------------|-------------------|-------------|-----------------------|---------------------|---------------------|----------------------|
| M01 | Single | kubeadm | kubeadm | false | N/A | N/A | N/A |
| M02 | Single | kubexm | kubexm | false | N/A | N/A | N/A |
| M03 | Single | kubeadm | kubexm | false | N/A | N/A | N/A |
| M04 | Multi | kubeadm | kubeadm | false | N/A | N/A | N/A |
| M05 | Multi | kubexm | kubexm | false | N/A | N/A | N/A |
| M06 | Multi | kubeadm | kubexm | false | N/A | N/A | N/A |
| M07 | Multi | kubeadm | kubeadm | true | internal | haproxy | static pod on workers |
| M08 | Multi | kubeadm | kubexm | true | internal | haproxy | static pod on workers |
| M09 | Multi | kubexm | kubexm | true | internal | haproxy | binary on workers |
| M10 | Multi | kubeadm | kubeadm | true | internal | nginx | static pod on workers |
| M11 | Multi | kubeadm | kubexm | true | internal | nginx | static pod on workers |
| M12 | Multi | kubexm | kubexm | true | internal | nginx | binary on workers |
| M13 | Multi | kubeadm | kubeadm | true | kube-vip | N/A | kube-vip |
| M14 | Multi | kubeadm | kubexm | true | kube-vip | N/A | kube-vip |
| M15 | Multi | kubexm | kubexm | true | kube-vip | N/A | kube-vip |
| M16 | Multi | kubeadm | kubeadm | true | external | kubexm-kh | keepalived+haproxy on LB nodes |
| M17 | Multi | kubeadm | kubeadm | true | external | kubexm-kn | keepalived+nginx on LB nodes |
| M18 | Multi | kubeadm | kubexm | true | external | kubexm-kh | keepalived+haproxy on LB nodes |
| M19 | Multi | kubeadm | kubexm | true | external | kubexm-kn | keepalived+nginx on LB nodes |
| M20 | Multi | kubexm | kubexm | true | external | kubexm-kh | keepalived+haproxy on LB nodes |
| M21 | Multi | kubexm | kubexm | true | external | kubexm-kn | keepalived+nginx on LB nodes |

## 4. Test Cases

### 4.1 Global Flags Tests
| TC-ID | Command | Description | Expected Result |
|-------|---------|-------------|-----------------|
| GL-01 | `kubexm --verbose version` | Verify verbose output | Debug level logs are printed to console |
| GL-02 | `kubexm --yes cluster create -f config.yaml` | Verify automatic confirmation | Command executes without prompting for user interaction |

### 4.2 Cluster Commands
| TC-ID | Command | Parameters | Description | Expected Result |
|-------|---------|------------|-------------|-----------------|
| CL-01 | `create` | `-f config.yaml --skip-preflight` | Create cluster skipping preflight checks | Cluster creation begins without preflight checks |
| CL-02 | `create` | `-f config.yaml --dry-run` | Simulate cluster creation | DAG is planned, validation passes, but no changes are applied |
| CL-03 | `create` | `-f config.yaml` | Create cluster with offline mode | Automatically detects `packages/` directory and installs via offline mode |
| CL-04 | `delete` | `-f config.yaml` | Delete an existing cluster | Nodes are drained, components are removed, and configuration is cleaned up |
| CL-05 | `delete` | `-f config.yaml --dry-run` | Simulate cluster deletion | Deletion plan is generated but not executed |
| CL-06 | `upgrade` | `-f config.yaml -t v1.24.3` | Upgrade cluster to target version | Control plane and worker nodes upgraded sequentially |
| CL-07 | `upgrade` | `-f config.yaml` | Upgrade cluster without version (Invalid) | Fails, prompting for required `-t` version |
| CL-08 | `backup` | `-f config.yaml -t all -o /backup/` | Backup entire cluster | PKI, ETCD, and configuration data are backed up to `/backup/` |
| CL-09 | `backup` | `-f config.yaml -t invalid_type` | Backup with invalid type | Fails or defaults safely to "all" based on implementation |
| CL-10 | `restore` | `-f config.yaml -b /backup/arch.tar.gz -s /backup/etcd.db` | Restore entire cluster | Validates snapshot path and restores data |
| CL-11 | `restore` | `-f config.yaml -b /backup/arch.tar.gz` | Restore without snapshot | Fails stating snapshot path is required |
| CL-12 | `health` | `-f config.yaml -c apiserver` | Check specific component health | Only apiserver is validated for readiness |
| CL-13 | `reconfigure` | `-f config.yaml -c kubelet --restart=true` | Reconfigure Kubelet and restart | Kubelet config updated and service restarted |

### 4.3 Node Commands
| TC-ID | Command | Parameters | Description | Expected Result |
|-------|---------|------------|-------------|-----------------|
| NO-01 | `list` | `-c mycluster --kubeconfig /path/to/kubeconfig` | List nodes in the cluster | Returns standard output listing node details |
| NO-02 | `get` | `node-1 -c mycluster -o json` | Get details of specific node | Returns node details formatted in JSON |
| NO-03 | `cordon` | `node-1 -c mycluster` | Cordon a node | Node is marked as unschedulable |
| NO-04 | `uncordon`| `node-1 -c mycluster` | Uncordon a node | Node is marked as schedulable |
| NO-05 | `drain` | `node-1 -c mycluster --force --ignore-daemonsets` | Drain a node completely | Node is safely drained, disregarding DaemonSets and enforcing eviction |

### 4.4 Certs Commands
| TC-ID | Command | Parameters | Description | Expected Result |
|-------|---------|------------|-------------|-----------------|
| CE-01 | `renew all` | `--yes` | Renew all cluster certificates | Both Kubernetes and ETCD certificates renewed successfully |
| CE-02 | `renew kubernetes-ca` | `--yes` | Renew K8s CA certificates | K8s CA certificate is recreated and distributed |
| CE-03 | `renew kubernetes-certs`| `--yes` | Renew K8s leaf certificates | Leaf certs renewed while retaining the existing CA |
| CE-04 | `check-expiration`| | Check expiration dates | Expiration for all certs displayed |

### 4.5 Registry Commands
| TC-ID | Command | Parameters | Description | Expected Result |
|-------|---------|------------|-------------|-----------------|
| RE-01 | `create` | `--yes` | Setup private registry | Private registry deployed successfully |
| RE-02 | `delete` | `--yes` | Remove private registry | Private registry components and data uninstalled |

### 4.6 Utility Commands (Download & ISO)
| TC-ID | Command | Parameters | Description | Expected Result |
|-------|---------|------------|-------------|-----------------|
| UT-01 | `download`| `-f config.yaml -o /tmp/offline/` | Download offline assets | Host validation is skipped; required binaries, images downloaded to path |
| UT-02 | `iso create`| `-f config.yaml -o custom.iso` | Create a custom ISO package | A bootable ISO bundle containing all necessary packages is built |

## 5. Architectural Constraints Test Cases
| TC-ID | Constraint | Description | Expected Result |
|-------|------------|-------------|-----------------|
| AR-01 | Step Idempotency | Execute `create cluster` pipeline twice | Second run skips completed steps without errors or side effects |
| AR-02 | Central Authority | Prevent 127.0.0.1 or localhost in `host.yaml` | Validation fails at Preflight stage |
| AR-03 | Layer Enforcement | Ensure Step logic does not use SSH directly | Source code review; SSH actions are strictly handled via Runner and Connector |
| AR-04 | Error Propagation | Simulate a recoverable SSH timeout | Runner implements retry logic before failing |
| AR-05 | Panic Safety | Trigger a panic within a Module Plan | `SafePlan` catches the panic, preventing pipeline crash, and gracefully reports failure |

## 6. Execution Flow Validation
Ensure that when executing pipelines, the Directed Acyclic Graph (DAG) executes modules and tasks in the strict, expected order. Example for cluster creation:
1. PreflightConnectivityModule
2. PreflightModule
3. InfrastructureModule
4. LoadBalancerModule
5. ControlPlaneModule
6. NetworkModule
7. WorkerModule
8. AddonModule
