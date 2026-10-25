# Project-authored model fixtures

Generate with `python scripts/data/prepare_model_fixtures.py`.

These tiny abstract meshes are authored by the generator in this repository, with no external binary inputs. `LicenseRef-EMAD-TestFixture` permits internal development, tests and synthetic simulation, including perpetual source retention. These fixture declarations do not authorize external redistribution or certify aerodynamics, sensors, terrain accuracy or physical behavior.

- `abstract-wing.glb`: one visual triangle, not a flight model.
- `marker.glb`: abstract marker geometry.
- `ground-tile.gltf`: two triangles with an embedded buffer, not measured terrain.

Coordinates use glTF metres and right-handed Y-up axes. Authoring source is the retained generator; provider/terms links in tests are explicitly non-resolving fixture references.
