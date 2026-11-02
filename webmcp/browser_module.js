// Primitive owns the native browser registration and cancellation boundary.
// Tool schemas, admission and effects remain with the application callback.
// Registration options (signal, exposedTo) reach the browser unchanged, so its
// own validation and same-origin default stay authoritative.
export async function registerTool(tool, options = {}) {
  const {signal} = options;
  signal?.throwIfAborted();
  const context = document.modelContext;
  if (typeof context?.registerTool !== 'function') return false;
  const execute = tool.execute;
  // Leave invalid callback admission to the browser's native contract.
  if (typeof execute !== 'function') {
    await context.registerTool(tool, options);
    return true;
  }
  await context.registerTool({...tool, execute: async (input, invocation) => {
    const execution = signal ? AbortSignal.any([signal, invocation.signal]) : invocation.signal;
    execution.throwIfAborted();
    const result = await execute(input, {signal: execution});
    execution.throwIfAborted();
    return result;
  }}, options);
  signal?.throwIfAborted();
  return true;
}
