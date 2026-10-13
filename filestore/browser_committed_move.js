// Primitive owns native browser storage execution. Generated error identities
// come from Primitive Core; applications retain record and ownership policy.
export const BrowserStoreErrorIdentity = Object.freeze({
  contract: __CONTRACT_IDENTITY__,
  source: __SOURCE_IDENTITY__,
  destination: __DESTINATION_IDENTITY__,
  cleanup: __CLEANUP_IDENTITY__,
  callback: __CALLBACK_IDENTITY__,
});

function storageError(identity, cause) {
  const error = new Error('Browser storage operation refused', {cause});
  error.primitiveIdentity = identity;
  return error;
}

/**
 * Read exactly two keys in one native transaction, ask synchronous policy for
 * the replacement value, then write the destination and delete the source.
 * Ownership of the supplied database handle transfers to this operation.
 * Policy refusal aborts without writes and preserves the exact thrown cause.
 * No request success resolves this promise: native commit/abort must finish.
 */
export function commitGuardedMove(options = {}) {
  return new Promise((resolve, reject) => {
    let database, storeName, sourceKey, destinationKey, decide;
    let transaction;
    let primary;
    let result;
    let wrote = false;
    let settled = false;

    function finish(error) {
      if (settled) return;
      settled = true;
      try { if (typeof database?.close === 'function') database.close(); }
      catch (cause) {
        const cleanup = storageError(BrowserStoreErrorIdentity.cleanup, cause);
        error = error ? new AggregateError([error, cleanup], 'Browser storage close failed') : cleanup;
      }
      if (error) reject(error); else resolve(result);
    }

    function abort(error) {
      if (primary === undefined) primary = error;
      try { transaction.abort(); }
      catch (cause) {
        // An inactive native transaction has already committed or aborted.
        // Its scheduled terminal event still owns settlement and handle close.
        primary = new AggregateError([primary, storageError(BrowserStoreErrorIdentity.destination, cause)], 'Browser storage abort failed');
      }
    }

    try { ({database, storeName, sourceKey, destinationKey, decide} = options); }
    catch (cause) { finish(storageError(BrowserStoreErrorIdentity.contract, cause)); return; }
    if (!database || typeof database.transaction !== 'function' || typeof database.close !== 'function' ||
        typeof storeName !== 'string' || storeName.length === 0 || typeof sourceKey !== 'string' || sourceKey.length === 0 ||
        typeof destinationKey !== 'string' || destinationKey.length === 0 || sourceKey === destinationKey || typeof decide !== 'function') {
      finish(storageError(BrowserStoreErrorIdentity.contract));
      return;
    }
    try {
      transaction = database.transaction(storeName, 'readwrite');
      transaction.oncomplete = () => finish(primary ?? (wrote ? undefined : storageError(BrowserStoreErrorIdentity.destination)));
      transaction.onabort = () => finish(primary ?? storageError(BrowserStoreErrorIdentity.destination, transaction.error));
      // A request's error is not terminal. The default native error handling
      // aborts the transaction; onabort observes rollback before settlement.
      transaction.onerror = () => {
        if (primary === undefined) primary = storageError(BrowserStoreErrorIdentity.destination, transaction.error);
      };
      const store = transaction.objectStore(storeName);
      const source = store.get(sourceKey);
      source.onerror = () => { primary ??= storageError(BrowserStoreErrorIdentity.source, source.error); };
      source.onsuccess = () => {
        let destination;
        try { destination = store.get(destinationKey); }
        catch (cause) { abort(storageError(BrowserStoreErrorIdentity.source, cause)); return; }
        destination.onerror = () => { primary ??= storageError(BrowserStoreErrorIdentity.source, destination.error); };
        destination.onsuccess = () => {
          try {
            result = decide({source: source.result, destination: destination.result});
            if (result === undefined || (result !== null && (typeof result === 'object' || typeof result === 'function') && typeof result.then === 'function')) {
              abort(storageError(BrowserStoreErrorIdentity.contract));
              return;
            }
          } catch (error) {
            abort(error instanceof Error ? error : storageError(BrowserStoreErrorIdentity.callback, error));
            return;
          }
          try {
            store.put(result, destinationKey);
            store.delete(sourceKey);
            wrote = true;
          } catch (cause) {
            abort(storageError(BrowserStoreErrorIdentity.destination, cause));
          }
        };
      };
    } catch (cause) {
      if (transaction) abort(storageError(BrowserStoreErrorIdentity.source, cause));
      else finish(storageError(BrowserStoreErrorIdentity.source, cause));
    }
  });
}
