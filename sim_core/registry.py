"""Unified Biome & Environment Registry shared across all EMAD simulators.

Automatically discovers:
1. Staged Riyadh urban pilot (enriched with GHS-OBAT heights, WorldPop 2024, CAMS dust).
2. Staged large-scale 24-30 km Saudi biomes under ``private/environment-foundation/sim-biomes/``
   (Riyadh Metro 24km, Asir Highlands 30km, Jubail Industrial Coast 30km).
3. Staged Jeddah Red Sea coastal biome (``jeddah-v2`` + GHS-OBAT + WorldPop).
4. Git-committed owned verification fixture (``owned-fixture-v1``).
"""
from __future__ import annotations

from dataclasses import dataclass
import json
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parents[1]


@dataclass(frozen=True)
class BiomePreset:
    id: str
    title: str
    biome: str  # urban_basin | mountain_highlands | industrial_coast | red_sea_coast | verification_fixture
    description: str
    package_path: Path
    expected_manifest_sha256: str | None
    weather_field_path: Path | None = None
    glb_path: Path | None = None
    imagery_path: Path | None = None
    scenario_template_path: Path | None = None
    ghs_obat_path: Path | None = None
    worldpop_path: Path | None = None
    gebco_path: Path | None = None
    cams_dust_path: Path | None = None


def _opt(path: Path) -> Path | None:
    return path if path.is_file() else None


def discover_biomes(repo_root: Path = REPO_ROOT) -> list[BiomePreset]:
    """Discover all available verified/staged environments on disk with companion enrichment sidecars."""
    priv = repo_root / "private/environment-foundation/implementation"
    worldpop_sa = _opt(priv / "riyadh-access-review/worldpop-2024.tif")
    ghs_sa_zip = _opt(priv / "riyadh-intake-v1/enrichment/ghs-obat-saudi.zip")
    ghs_riyadh = _opt(priv / "riyadh-intake-v1/enrichment/ghs-obat-riyadh.geojson")
    cams_riyadh = _opt(priv / "supporting-source-review/cams-dust.json")
    gebco_jubail = _opt(priv / "coastal-moving-qualification/gebco-capture-jubail-v1.zip")
    era5_fallback = _opt(priv / "supporting-source-review/era5-next/era5-pressure-subset.json")

    presets: list[BiomePreset] = []

    # 1. Riyadh 6x6 km Reference Area (Enriched with GHS-OBAT + WorldPop + CAMS)
    riyadh_pkg = priv / "riyadh-observation-pilot/staged-prepared.zip"
    if riyadh_pkg.is_file():
        presets.append(
            BiomePreset(
                id="riyadh-observation-v1",
                title="Riyadh Reference Area (GLO-30 + GHS-OBAT + WorldPop + ERA5)",
                biome="urban_basin",
                description=(
                    "Staged 6x6 km Riyadh urban package: Copernicus GLO-30 DSM, EGM2008 geoid, "
                    "1,978 OSM buildings fused with GHS-OBAT heights, 3,703 roads, WorldPop 2024 "
                    "population density, CAMS dust, Sentinel-2 drape, and embedded ERA5 winds."
                ),
                package_path=riyadh_pkg,
                expected_manifest_sha256="175515f785c2ae6cdbbec0303140f6d425ff8e773dc24375a64d02d87ac63ab4",
                weather_field_path=None,
                glb_path=_opt(priv / "riyadh-observation-pilot/staged-surface.glb"),
                imagery_path=_opt(priv / "riyadh-observation-pilot/staged-imagery.json"),
                scenario_template_path=_opt(repo_root / "sim/scenarios/riyadh_intercept.json"),
                ghs_obat_path=ghs_riyadh or ghs_sa_zip,
                worldpop_path=worldpop_sa,
                gebco_path=None,
                cams_dust_path=cams_riyadh,
            )
        )

    # 2. Large-Scale 24-30 km Saudi Biomes (auto-discovered from private/environment-foundation/sim-biomes/*)
    biomes_dir = repo_root / "private/environment-foundation/sim-biomes"
    if biomes_dir.is_dir():
        for meta_path in sorted(biomes_dir.glob("*/biome.json")):
            try:
                meta = json.loads(meta_path.read_text(encoding="utf-8"))
                folder = meta_path.parent
                pkg = folder / meta.get("package", "prepared.zip")
                if not pkg.is_file():
                    continue
                ghs_local = _opt(folder / "ghs-obat.geojson") or ghs_sa_zip
                gebco_local = _opt(folder / "gebco.zip") or (gebco_jubail if "jubail" in meta["id"] else None)
                weather_local = _opt(folder / "era5-pressure.json") or (era5_fallback if "riyadh" in meta["id"] else None)
                presets.append(
                    BiomePreset(
                        id=str(meta["id"]),
                        title=str(meta["title"]),
                        biome=str(meta.get("biome", "urban_basin")),
                        description=str(meta.get("description", "")),
                        package_path=pkg,
                        expected_manifest_sha256=meta.get("manifestSha256"),
                        weather_field_path=weather_local,
                        glb_path=_opt(folder / meta.get("glb", "surface.glb")),
                        imagery_path=_opt(folder / meta.get("imagery", "imagery.json")),
                        scenario_template_path=_opt(folder / "scenario.json"),
                        ghs_obat_path=ghs_local,
                        worldpop_path=worldpop_sa,
                        gebco_path=gebco_local,
                        cams_dust_path=cams_riyadh if "riyadh" in meta["id"] else None,
                    )
                )
            except Exception:
                continue

    # 3. Jeddah Red Sea Coastal Biome (already staged offline!)
    jeddah_pkg = priv / "jeddah-v2/prepared.zip"
    if jeddah_pkg.is_file():
        presets.append(
            BiomePreset(
                id="jeddah-coast-v1",
                title="Jeddah Red Sea Coast (GLO-30 + OSM + GHS-OBAT + WorldPop)",
                biome="red_sea_coast",
                description=(
                    "Staged 8x8 km Jeddah Red Sea coastal urban biome with Copernicus GLO-30 DSM, "
                    "EGM2008 geoid, OSM buildings & roads, GHS-OBAT heights, and WorldPop 2024 exposure."
                ),
                package_path=jeddah_pkg,
                expected_manifest_sha256="38ac7fd41b4ae55de8d982e2e5f657c6569e663212253afb29bbd90db69b6d52",
                weather_field_path=None,  # Uses synthetic coastal fallback if ERA5 doesn't cover Red Sea
                glb_path=_opt(priv / "jeddah-v2/surface.glb"),
                imagery_path=_opt(priv / "jeddah-v2/imagery.json"),
                scenario_template_path=None,
                ghs_obat_path=_opt(priv / "jeddah-v2/ghs-obat.geojson") or ghs_sa_zip,
                worldpop_path=worldpop_sa,
                gebco_path=None,
                cams_dust_path=None,
            )
        )

    # 4. Owned Verification Fixture (CI regression suite)
    fixture_pkg = repo_root / "testdata/geospatial/owned-fixture-v1/prepared.zip"
    if fixture_pkg.is_file():
        presets.append(
            BiomePreset(
                id="owned-fixture-v1",
                title="Owned Fixture Verification Environment",
                biome="verification_fixture",
                description=(
                    "Git-committed owned verification environment with multi-component surface rasters, "
                    "sample buildings/roads, and companion ERA5 pressure-level wind field."
                ),
                package_path=fixture_pkg,
                expected_manifest_sha256="aaf9f14b027749b5d233018606fb90c81e34808b9522e034fc67b5243c966bdc",
                weather_field_path=_opt(repo_root / "testdata/geospatial/owned-fixture-v1/field-era5.json"),
                glb_path=None,
                imagery_path=None,
                scenario_template_path=_opt(repo_root / "sim/scenarios/fixture_intercept.json"),
                ghs_obat_path=None,
                worldpop_path=None,
                gebco_path=None,
                cams_dust_path=None,
            )
        )

    return presets
