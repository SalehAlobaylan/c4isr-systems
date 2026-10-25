// Mocked presentation suites still use the same explicit identity/scope shape
// as the real-backend gate. Production never infers authority from these values.
export function mockSession(headers: Record<string, string> = {}) {
  const resource = { workspaceId: headers['x-emad-workspace-id'] ?? 'default', environmentId: headers['x-emad-environment-id'] ?? 'local', partition: headers['x-emad-partition'] ?? 'EXERCISE_OBSERVATION', runId: headers['x-emad-run-id'] ?? 'legacy' }
  return {
    operator: { id: 'operator-01', name: 'Operator 01', role: 'operator' },
    principal: { id: 'human:operator-01', kind: 'human' }, resource, resources: [resource], accessRevision: 'fixture-access-v1',
    permissions: ['operational', 'sources', 'observations', 'tracks', 'assets', 'missions', 'commands', 'classifications', 'geofences', 'alerts', 'incidents', 'assessments', 'audit', 'scenarios', 'operators'].map((p) => `${p}.read`).concat(['scenarios.control', 'sim.scenarios.author']),
  }
}
