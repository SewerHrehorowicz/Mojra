"use strict";

// NodeCanvasVertexShader: GLSL vertex shader source for the WebGL instanced node canvas.
const NodeCanvasVertexShader = {
  source: `
    attribute vec2 aUnit;
    attribute vec4 aRect;
    attribute vec4 aUV;
    attribute vec4 aColor;
    uniform vec2 uViewSize;
    uniform vec3 uPanZoom;
    varying vec2 vUV;
    varying vec4 vColor;
    varying float vMode;
    varying float vPxRange;
    const float uvGridTest = -80000.0;
    const float uvGridOffset = 200000.0;
    const float uvScreenTest = -30000.0;
    const float uvSolid = 30.0;
    const float uvFontLg = 20.0;
    const float uvFontSm = 10.0;
    void main() {
      vColor = aColor;
      vMode = 0.0;
      vPxRange = 4.5;
      vec2 uvs = mix(aUV.xy, aUV.zw, aUnit);
      if (aUV.y < -0.5 && aRect.w > aRect.z) {
        uvs.x = mix(aUV.x, aUV.z, aUnit.y);
      }
      vUV = uvs;
      vec2 screenPos;
      if (aUV.x < uvGridTest) {
        screenPos = aUnit * uViewSize;
        vec2 worldPos = (screenPos - uPanZoom.xy) / uPanZoom.z;
        vUV = -worldPos - vec2(uvGridOffset);
      } else if (aUV.x < uvScreenTest) {
        screenPos = aRect.xy + aUnit * aRect.zw;
      } else {
        if (aUV.x >= 25.0) {
          vUV.x = uvs.x - uvSolid;
          vMode = -1.0;
        } else if (aUV.x >= 15.0) {
          vUV.x = uvs.x - uvFontLg;
          vMode = 1.0;
          vPxRange = 3.39;
        } else if (aUV.x >= 5.0) {
          vUV.x = uvs.x - uvFontSm;
          vMode = 1.0;
          vPxRange = 2.80;
        }
        vec2 worldPos = aRect.xy + aUnit * aRect.zw;
        screenPos = worldPos * uPanZoom.z + uPanZoom.xy;
      }
      vec2 clip = (screenPos / uViewSize) * 2.0 - 1.0;
      gl_Position = vec4(clip.x, -clip.y, 0.0, 1.0);
    }
  `
};
