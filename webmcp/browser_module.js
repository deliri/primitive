// Primitive owns the native browser registration and cancellation boundary.
// Tool schemas, admission and effects remain with the application callback.
export async function registerTool(tool, {signal} = {}) {
  signal?.throwIfAborted();
  const context = document.modelContext;
  if (typeof context?.registerTool !== 'function') return false;
  const execute = tool.execute;
  // Leave invalid callback admission to the browser's native contract.
  if (typeof execute !== 'function') {
    await context.registerTool(tool, {signal});
    return true;
  }
  await context.registerTool({...tool, execute: async (input, options) => {
    const execution = signal ? AbortSignal.any([signal, options.signal]) : options.signal;
    execution.throwIfAborted();
    const result = await execute(input, {signal: execution});
    execution.throwIfAborted();
    return result;
  }}, {signal});
  signal?.throwIfAborted();
  return true;
}
