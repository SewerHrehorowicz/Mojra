"use strict";

// NodeCanvasFragmentShader: GLSL fragment shader source for the WebGL instanced node canvas.
const NodeCanvasFragmentShader = {
  source: `
    precision highp float;
    varying vec2 vUV;
    varying vec4 vColor;
    varying float vMode;
    varying float vPxRange;
    uniform sampler2D uUIAtlas;
    uniform sampler2D uFontAtlas;
    uniform vec3 uBgColor;
    uniform vec3 uDotColor;
    uniform float uGridStep;
    uniform vec3 uPanZoom;
    uniform float uDpr;
    const float uvGridTest = -80000.0;
    const float uvGridOffset = 200000.0;
    const float uvDashDuty = 0.54;
    float median(float redChannel, float greenChannel, float blueChannel) {
      return max(min(redChannel, greenChannel), min(max(redChannel, greenChannel), blueChannel));
    }
    void main() {
      if (vUV.x < uvGridTest) {
        vec2 worldPos = -vUV - vec2(uvGridOffset);
        vec2 cell = fract((worldPos + vec2(0.8)) / uGridStep) * uGridStep;
        float dotMask = step(cell.x, 1.7) * step(cell.y, 1.7);
        gl_FragColor = vec4(mix(uBgColor, uDotColor, dotMask), 1.0);
        return;
      }
      if (vUV.y < -0.5) {
        float dist = vUV.x;
        float period = -vUV.y;
        if (fract(dist / period) > uvDashDuty) discard;
        gl_FragColor = vColor;
        return;
      }
      if (vMode < -0.5) {
        gl_FragColor = texture2D(uUIAtlas, vUV) * vColor;
        return;
      }
      vec4 tex = (vMode > 0.5) ? texture2D(uFontAtlas, vUV) : texture2D(uUIAtlas, vUV);
      float signedDist = median(tex.r, tex.g, tex.b) - 0.5;
      float screenPxRange = max(1.5, vPxRange * uPanZoom.z * uDpr);
      float alpha = clamp(signedDist * screenPxRange + 0.5, 0.0, 1.0);
      if (alpha <= 0.01) discard;
      gl_FragColor = vec4(vColor.rgb, vColor.a * alpha);
    }
  `
};
