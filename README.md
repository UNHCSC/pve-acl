![Tests](https://img.shields.io/github/actions/workflow/status/UNHCSC/pve-acl/ci.yml?branch=main&event=push&label=CI)
![Made with Golang](https://img.shields.io/badge/-Made_with_Golang-007d9c?logo=go&logoColor=white)

# Organesson Cloud

Organesson Cloud is a Proxmox-backed access control system that uses LDAP and a local database to create fine-tuned asset management on a per-user basis without clogging up your Proxmox cluster.

## Motivation

My problem is that I have a class to TA next semester, and I want to give students access to Proxmox VMs (sometimes more than one per student, even an entire virtualized network!) without giving them access to the Proxmox cluster itself. However, they should still have some ability to control VMs they "own". Namely, I want students to be able to do a few things:
- Start and stop VMs
- View VM consoles/KVMs
- View VM resource usage (CPU, RAM, disk, network)

I also want non-students (e.g. other TAs, professors) to be able to manage creation of VMs in pools and assign them to students, but I don't want them to have access to the Proxmox cluster itself either. Finally, I want to be able to easily manage all of this without having to mess with Proxmox's ACL system, which is very powerful but also very complex and not designed for this use case.

## Goals

1. Have modular access control system that has "Domain Admins" at the top and they can use LDAP groups to assign permissions to users (or groups).
2. Assets (Proxmox Networks, VMs, Containers, etc.) are assigned to users and or groups, and users can only see and manage assets that are assigned to them.
3. Users can only perform actions on assets that they have permissions for, and these permissions are defined in a local database that is separate from Proxmox's ACL system. Note that there should be a native console/vnc viewer that users can use to access their VMs without needing to log into Proxmox itself.
4. The system should be easy to manage and scale, and should not require a lot of manual configuration in Proxmox itself. (Only an API user/token will be necessary)

## Proxmox inventory safety

Organesson's discovery is read-only and retains only guests carrying the exact configured `managed_tag`, which defaults to `organesson-managed`. Untagged guests are never adopted, persisted as new inventory, or operated on. Nodes, storage, and networks are displayed only as cluster context.

The real-cluster smoke test is disabled by default and performs GET requests only. Configure the `[proxmox]` section in the ignored local `config.toml`, then run. Keep `verify_tls = true` for a publicly or locally trusted certificate. For a private Proxmox CA, set `tls_fingerprint_sha256` to the expected leaf-certificate fingerprint; hostname validation remains enforced with the pin.

```sh
ORGANESSON_PROXMOX_SMOKE=1 go test ./proxmox -run TestRealClusterReadOnlyInventory -v
```
