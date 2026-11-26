"""EMAD 3D Simulation Core.

Reusable, deterministic simulation modules operating directly on verified EMAD
geographic environment packages (prepared.zip), preserving strict separation
between EXERCISE_INPUT, EXERCISE_TRUTH, EXERCISE_OBSERVATION, and EVALUATION.
"""

from .contracts import (
    FIDELITY_KINEMATIC_3DOF,
    SCOPE_EVALUATION,
    SCOPE_EXERCISE_INPUT,
    SCOPE_EXERCISE_OBSERVATION,
    SCOPE_EXERCISE_TRUTH,
)

__all__ = [
    "FIDELITY_KINEMATIC_3DOF",
    "SCOPE_EXERCISE_INPUT",
    "SCOPE_EXERCISE_TRUTH",
    "SCOPE_EXERCISE_OBSERVATION",
    "SCOPE_EVALUATION",
]
