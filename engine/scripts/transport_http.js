"use strict";

const TransportHTTP = {
  defaultEndpoint: "/api/action",
  methodPost: "POST",
  headerContentType: "Content-Type",

  async send(endpoint, contentType, payloadBytes) {
    const resp = await fetch(endpoint || this.defaultEndpoint, {
      method: this.methodPost,
      headers: { [this.headerContentType]: contentType },
      body: payloadBytes
    });
    if (!resp || !resp.ok) {
      return null;
    }
    return resp.arrayBuffer();
  }
};
