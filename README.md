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

## Go Coding Guidelines

- Important functions should have a brief one-line comment describing what they do, and more detailed comments if necessary.
- Use clear and descriptive variable names.

Follow these style guidelines:

```go

// Documentation comment....
func (s *Struct) ImportantFunction(a, b int, c string) (result int, err error) { // Return types should be named
    if a < 0 || b < 0 {
        err = fmt.Errorf("a and b must be non-negative, got a=%d, b=%d", a, b) // Declare return values before returning
        return // Naked returns are acceptable in this case since we have named return values
    } // A full line of space after closing a block for readability and flow

    if result = a + b; result > 100 { // If we can condense two related statements into one in an if statement, we should do so for readability and flow
        err = fmt.Errorf("result must be less than or equal to 100, got %d", result)
        return
    }

    return
}

func main() {
    var ( // Variable blocks for readability, avoid := at all costs EXCEPT for loops or case statements
        a, b, result int = 10, 20, 0
        err    error
    )

    if result, err = ImportantFunction(a, b, "example"); err != nil {
        log.Fatalf("Error calling ImportantFunction: %v", err)
    }

    fmt.Printf("Result: %d\n", result)
}
```