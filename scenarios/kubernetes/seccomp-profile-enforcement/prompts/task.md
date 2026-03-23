# Task: Apply a Seccomp profile to restrict dangerous syscalls

The `syscall-tester` pod in the `bench` namespace is running with
`seccompProfile.type: Unconfined`, which allows all syscalls including
dangerous ones like `unshare` and `mount`.

Your tasks:

1. Investigate the `syscall-tester` pod and confirm it runs without Seccomp restrictions
2. Apply an appropriate Seccomp profile to restrict dangerous syscalls — either:
   - `RuntimeDefault` (the standard Seccomp profile that blocks dangerous syscalls), or
   - A custom `Localhost` profile if you need finer control
3. Verify the pod restarts with the Seccomp profile active
4. Confirm the application still works (it runs a simple web server that should not need privileged syscalls)

The baseline `web` deployment must remain healthy throughout.
