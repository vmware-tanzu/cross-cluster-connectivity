#8 Client Side Orphan Service Record Deletion

Date 2020-11-11
Author: Jamie Monserrate, Tyler Schultz

## Status

Accepted

## Context

Imported ServiceRecords (client side ServiceRecords) will become orphaned when a
delete message is missed. This may occur if the client were to become
disconnected from the server when the delete occurred. The delete message would
not be delivered and the RegistryClient would not know that an imported
ServiceRecord is no longer valid.

Upon a client establishing a new connection or on redial, the hamlet protocol
specifies that all Records are to be sent to the client. There is no explicit
message / procedure call to indicate that the bulk sync has completed.  Messages
received after the bulk sync are indistinguishable from messages received during
the bulk sync. Therefore, there is no way to know when it is safe to delete
ServiceRecords for which has not been messaged.

## Proposed Solution

1. When the registry client starts, record in memory the ServiceRecords that are
   received from the registry server. After a delay of 5mins (arbitrary) time,
   list all of the imported ServiceRecords in the kube api, determine the list
   of ServiceRecords that were not received, and then delete the orphan
   ServiceRecords. The delay timer is only started upon the receiving the
   initial ServiceRecord from the registry server. If the registry client were
   to disconnect or redial, cancel the delayed delete action and start the whole
   process over again.

A danger of this approach is if for some reason the registry server were slow to
send the state of the world, the client would improperly delete ServiceRecords
that are still valid. This solution assumes the server is healthy and is able to
message the exported ServiceRecords in a timely fashion.

## Alternatives Considered

1. Using a timer to deduce when the bulk sync has ended - Immediately following
   a redial, start a countdown timer whenever a ServiceRecord is received. When
   another ServiceRecord is received, restart the timer. The timer is renewed to
   a time that is longer than the time between messages when the bulk sync is
   happening. When the bulk sync is over the timer will elapse, indicating that
   any ServiceRecords that have not received a corresponding message are safe
   for deletion. Possible problems with this approach include falsely deducing
   that the bulk sync is over and then deleting valid ServiceRecords, or
   conversely falsely deducing the bulk sync is continuing when it’s actually
   over and not deleting orphaned ServiceRecords.

1. Drop all Service Records on the Client Side prior to a redial - On a new
   connection or when attempting a redial, delete all the ServiceRecords on the
   client cluster, and then rely on receiving all ServiceRecords from the server
   to recreate the ServiceRecords. The major drawback of this approach is the
   downtime when the ServiceRecord is deleted. If the connect / redial were to
   fail, this would exacerbate the downtime - even though the services are
   available.

1. Add a “special” service record that indicates this is the end of the list -
   Include a special ServiceRecord message that indicates the end of the bulk
   sync. Client side ServiceRecords not received before the special
   ServiceRecord can safely be deleted. This breaks the hamlet protocol, and
   clients/servers not accustomed to this message would be broken by these
   special messages. This approach also assumes that messages will be received
   in the order they are sent, which is not indicated by the protocol.

1. Deduce bulk sync has ended when updates trickle in - Creates are sent during
   bulk sync, and updates are sent during steady state. So we could start
   deleting orphans upon the first update message. This is not a viable option
   because this is an implementation detail of the cross-cluster-connectivity
   libraries, and not part of the hamlet protocol. Another issue with this
   approach is that no updates may happen for an extended period of time,
   prolonging the life of orphan ServiceRecords.

1. Moving away from Hamlet and using Kube APIs directly - This option feels too
   expensive to migrate to, locks us in to Kubernetes only, and would require
   the registry client to have credentials to the kube API for a service
   cluster.

1. When the registry client reconnects assume that every Service Record is a
   potential orphan. As the registry client receives events for Service Records,
   remove them from the potential orphan. After X amount of time, delete
   everything in the potential orphan list.

## Related Reading
https://github.com/vmware/hamlet/blob/master/spec/service-discovery.md
