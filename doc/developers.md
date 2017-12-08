developer documentation
=======================

components
----------

### observer

- **configure with:** k8s cluster credentials, `kubectl` command.
- **consume:** clock ticks.
- **produce:** `observedState` snapshots.

An `observedState` snapshot is a big heap of state more or less raw from kubernetes.
It's all the first class and second class k8s objects in a namespace.

TODO: We say 'snapshot', but can we, really?  *Should we speak with etcd directly?*

### intentioner

- **configure with:** your entire set of kubernetes yaml files.
- **consume:** human impulse (or clock ticks, or git pulls in practice)
- **produce:** `userIntent` set.

### gapAnalyzer

- **configure with:** nothing?
- **consume:** `observedState` snapshot plus `userIntent` set.
- **produce:** `gapAnalysis` plus `gapBrownlist(?)`.

A `gapAnalysis`

- Mark fields grey which are known to rekkoner as mutationy.
- Mark fields grey which are brownlisted as mutationy in userIntent.
- Mark fields red or green which are set in userIntent, depending on if they match.
- Mark fields yellow which are left in the `observedState` -- these are scary unknown changes.
- Mark fields yellow which are left in the `userIntent` set -- these need to be added.

This same marking process evaluates on whole objects as well, with one
additional step next-to-last: if objects declare a 'metadata.ownerReferences',
then they can be marked grey because they're someone else's problem.

### annealStepPlanner

TODO: this might not be a separate step... you discover the things that need
pushing during the gap analysis itself, and it's all phrased in terms of either
object identifiers in the observedState or filenames in the userIntent.
