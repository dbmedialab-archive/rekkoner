
things to observe
-----------------

### first class

These are easy because generally speaking nothing else creates them programmatically:

- deployments
- services
- jobs
- statefulSets
- daemonSets
- cronjobs

### second class

These are much harder because we need to make sure nothing else generated them:

- pods

(TODO: surely there are annotations we can take a hint from.)
(Yes: [metadata.ownerReferences](https://kubernetes.io/docs/concepts/workloads/controllers/garbage-collection/).)
(This built-in feature is kind of wild: it will attempt to "adopt" things automatically as of 1.8.)

### third class

These are almost *certainly* generated (and hopefully also cleaned up by) something else:

- replicaSets -- deployments use them internally; we can more or less ignore them

### totally disregarded

- replicationControllers -- they're depreciated in favor of deployments; we don't support them



how to handle objects "generically"
----------------------------------

`k8s.io/apimachinery/pkg/runtime.Unstructured` might be the thing to use.
Or possibly some factory that returns `Object` from that package.
I literally have no idea how to create a concrete instance of those though; surely
simple map-backed types that already implement these interfaces should exist, since
that's *literally what all this is* at the end of the day.

`k8s.io/apimachinery/pkg/apis/meta/v1.OwnerReference` appears to be the serial
format for the owner json that gets slammed into an annotation string.



partially controlled objects
----------------------------

Some objects in k8s are a mixture of declarative setup and state they have accumulated.
Unfortunately, we have to know the difference, because the former we should mutate
and delete freely, and the latter not so much.

Known problematic fields:

- `metadata.ownerReferences` -- will be updated by the cluster as of 1.8 -- see https://kubernetes.io/docs/concepts/workloads/controllers/garbage-collection/
- `metadata.finalizers` -- see https://kubernetes.io/docs/concepts/workloads/controllers/garbage-collection/
- `deletionTimestamp` -- nuff said

It would seem that we could handle this by the simple route of "if it's specified
in the declared intention files, then operate on it", but it's not so: this will lead you
into the spiked pit trap of incomplete-worldview-causes-memory-leaks all over again.

Expect to find more of these over time.

Maybe we should make a default brownlist of properties we *know* the common parts
of the k8s tools and community will make stateful messes in, and hardcode that
into our tool, and also have support for each object in the user's intents to
have a config tree dangling on the side that lists other properties to ignore.
Other properties changes generate yellow lights in the dashboard with a "nuke" button
but also don't trigger nuking by default because so much of k8s leans on these
stateful scantily-documented string bags.


making a believably useful dashboard
------------------------------------

The default tables emitted by e.g. `kubectl get pods --all-namespaces` seem
almost intentionally obtuse:

```
NAMESPACE        NAME                                 READY     STATUS              RESTARTS   AGE
default          demoapp-7c68d89475-khvlc             0/1       ImagePullBackOff    0          19h
```

Does that tell us literally ANYTHING useful about how fucked that pod is, or why?
No.  No it does not.

Bare minimum: let's make a pact with our future selves to have the sheer respect to at least
list *when* the last status update was?
