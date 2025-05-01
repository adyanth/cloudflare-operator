## Deleting a tunnel

Remember the delete order while deleting the tunnel. 
If you delete the secrets before deleting the tunnel, the operator won't be able to clean up the tunnel from Cloudflare, since it no longer has the credentials for the same.

The correct order is to delete the tunnel, wait for it to actually get deleted (after the finalizer is removed) and then delete the secret.
