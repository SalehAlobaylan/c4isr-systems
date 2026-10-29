import { resolve } from 'node:path'

// Repeated verification must not overwrite an earlier candidate's evidence.
export const testEvidencePath = (name: string) => resolve(process.env.EMAD_E2E_EVIDENCE_DIR ?? 'test-results/map-first', name)
