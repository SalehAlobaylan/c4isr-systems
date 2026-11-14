"""Combine registered native surface grids without resampling or filling gaps."""
from contextlib import ExitStack

import numpy as np
import rasterio
from affine import Affine


def native_mosaic(members, limit):
    datum = members[0][0]['details']['verticalDatum']
    with ExitStack() as resources:
        tiles = [(member, resources.enter_context(rasterio.open(path))) for member, path in members]
        first = tiles[0][1]
        transform = first.transform
        registration = first.tags().get('AREA_OR_POINT', 'Area')
        rectangles = []
        for member, tile in tiles:
            grid = tile.transform
            if (tile.crs is None or tile.crs.to_epsg() != 4326 or tile.count != 1 or grid.b or grid.d
                    or grid.a <= 0 or grid.e >= 0 or grid.a != transform.a or grid.e != transform.e
                    or tile.tags().get('AREA_OR_POINT', 'Area') != registration
                    or member['details']['verticalDatum'] != datum):
                raise ValueError('Surface tiles require aligned north-up WGS84 grids, registration and datum')
            col, row = (~transform) * (grid.c, grid.f)
            x, y = round(col), round(row)
            if max(abs(col - x), abs(row - y)) > 1e-7:
                raise ValueError('Surface tile boundaries are offset from the native grid')
            rectangles.append((x, y, x + tile.width, y + tile.height))

        left = min(r[0] for r in rectangles)
        top = min(r[1] for r in rectangles)
        width = max(r[2] for r in rectangles) - left
        height = max(r[3] for r in rectangles) - top
        if min(width, height) < 2 or width * height > limit:
            raise ValueError('Native surface mosaic exceeds the display sample budget')
        mosaic = np.full((height, width), np.nan)
        for (_, tile), (x, y, right, bottom) in zip(tiles, rectangles):
            values = tile.read(1, masked=True).astype(np.float64).filled(np.nan) * tile.scales[0] + tile.offsets[0]
            target = mosaic[y - top:bottom - top, x - left:right - left]
            valid = np.isfinite(values)
            common = valid & np.isfinite(target)
            if np.any(target[common] != values[common]):
                raise ValueError('Surface overlap has conflicting native heights')
            target[valid] = values[valid]
    return mosaic, transform * Affine.translation(left, top), datum, registration
