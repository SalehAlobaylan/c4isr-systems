// Exact shared data for browser and no-GPU scene tests, in ENU metres.
const terrain=[]
for(let n=0;n<3;n++)for(let e=0;e<3;e++)terrain.push({x:(e-1)*200,y:(n-1)*200,z:e===1&&n===1?10:0})
export const fixture={assets:[{file:'ground-tile.gltf',local:{x:0,y:0,z:0}},{file:'abstract-wing.glb',local:{x:0,y:0,z:10}},{file:'marker.glb',local:{x:100,y:100,z:0}}],terrain,triangles:[0,1,3,1,4,3,1,2,4,2,5,4,3,4,6,4,7,6,4,5,7,5,8,7],coordinates:[{x:0,y:0,z:100},{x:111.3194907876,y:0,z:-.0009714458},{x:9999.123456789,y:-9756.87654321,z:123.0123456789}]}
