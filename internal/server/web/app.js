(() => {
  const $ = (id) => document.getElementById(id);
  const feed = $("feed");
  const status = $("status");
  const countEl = $("count");
  let socket = null;
  let seen = 0;

  function wsURL() {
    const scheme = location.protocol === "https:" ? "wss:" : "ws:";
    return `${scheme}//${location.host}`;
  }

  function buildFilter() {
    const f = {};
    const kinds = $("kinds").value.trim();
    if (kinds) f.kinds = kinds.split(",").map((x) => parseInt(x.trim(), 10));
    const authors = $("authors").value
      .split("\n").map((x) => x.trim()).filter(Boolean);
    if (authors.length) f.authors = authors;
    const search = $("search").value.trim();
    if (search) f.search = search;
    f.limit = parseInt($("limit").value || "100", 10);
    return f;
  }

  function setStatus(text, cls) {
    status.textContent = text;
    status.className = "badge " + (cls || "");
  }

  function addMsg(cls, meta, body) {
    const empty = feed.querySelector(".empty");
    if (empty) empty.remove();
    const div = document.createElement("div");
    div.className = "msg " + cls;
    div.innerHTML =
      `<div class="meta">${meta}</div><div class="content">${body}</div>`;
    feed.prepend(div);
  }

  function esc(s) {
    return String(s).replace(/[&<>]/g, (c) =>
      ({ "&": "&amp;", "<": "&lt;", ">": "&gt;" }[c]));
  }

  function pretty(obj) {
    const json = JSON.stringify(obj, null, 2);
    return esc(json)
      .replace(/"([^"]+)":/g, '<span class="k">"$1"</span>:')
      .replace(/: "([^"]*)"/g, ': <span class="s">"$1"</span>')
      .replace(/: (\d+)/g, ': <span class="n">$1</span>');
  }

  function onMessage(evt) {
    let arr;
    try { arr = JSON.parse(evt.data); } catch { return; }
    const type = arr[0];
    switch (type) {
      case "EVENT": {
        const sub = arr[1], e = arr[2];
        seen++;
        countEl.textContent = `${seen} events`;
        const when = new Date(e.created_at * 1000).toLocaleTimeString();
        addMsg("event",
          `sub <span class="kind">${esc(sub)}</span> · kind ` +
          `<span class="kind">${e.kind}</span> · ${esc(e.pubkey.slice(0, 12))}… · ${when}`,
          esc(e.content || "") + "\n\n" + pretty(e));
        break;
      }
      case "OK":
        addMsg(arr[2] ? "ok" : "err",
          `OK · ${esc(arr[1].slice(0, 12))}… · ${arr[2]}`, esc(arr[3] || ""));
        break;
      case "EOSE":
        addMsg("sys", "EOSE", `subscription <span class="kind">${esc(arr[1])}</span> synced`);
        break;
      case "NOTICE":
        addMsg("err", "NOTICE", esc(arr[1]));
        break;
      case "CLOSED":
        addMsg("err", "CLOSED", `${esc(arr[1])} — ${esc(arr[2])}`);
        break;
      case "AUTH":
        addMsg("sys", "AUTH challenge", `<code>${esc(arr[1])}</code>`);
        break;
    }
  }

  function connect() {
    if (socket) socket.close();
    seen = 0;
    countEl.textContent = "0 events";
    const url = wsURL();
    socket = new WebSocket(url);
    setStatus("connecting…");
    socket.onopen = () => {
      setStatus("connected", "ok");
      socket.send(JSON.stringify(["REQ", "console", buildFilter()]));
    };
    socket.onclose = () => setStatus("disconnected", "bad");
    socket.onerror = () => setStatus("error", "bad");
    socket.onmessage = onMessage;
  }

  $("connect").onclick = connect;
  $("disconnect").onclick = () => socket && socket.close();
  $("clear").onclick = () => {
    feed.innerHTML = '<div class="empty">Feed cleared.</div>';
    seen = 0;
    countEl.textContent = "0 events";
  };
  $("send").onclick = () => {
    if (!socket || socket.readyState !== 1) {
      addMsg("err", "client", "not connected");
      return;
    }
    const text = $("raw").value.trim();
    try {
      JSON.parse(text); // validate
      socket.send(text);
    } catch (e) {
      addMsg("err", "client", "invalid JSON: " + e.message);
    }
  };

  // Show relay info on load.
  fetch("/").then((r) => r.json().catch(() => null)).then((info) => {
    if (info && info.name) $("relayinfo").textContent = `${info.name} · NIPs ${(info.supported_nips||[]).length}`;
  }).catch(() => {});

  connect();
})();
