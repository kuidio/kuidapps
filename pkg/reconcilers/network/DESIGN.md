# design

utlimately the network reconciler is the uber reconciler
it checks the network devices and check the differences -> this indicates there was a change

Now the question remains:
- what to do here?
    - automatic mode: update the CR
    - gitops mode: how to reopen an approved package revision -> trigger a new PVAR


PVAR -> PV -> KFORM -> NETWORK

NETWORK
- NETWORKPARAM
- NETWORKDEVICES
- NETWORKDEVICESPROVIDER