// Lightweight input inspector. Receives real terminal input through a local adapter.
export async function installTouchTrackingDemo(terminal) {
  const byId = id => document.getElementById(id);
  const counts = { press: 0, drag: 0, release: 0, motion: 0, up: 0, down: 0, native: 0, untrusted: 0, prevented: 0, allowed: 0, scroll: 0 };
  let tracking = true;
  let touches = 0;
  let last = null;
  let nativeCell = null;
  let pending = false;
  let log = [];
  let zoom = 0;
  let draw = '';
  let output;
  let held = false;

  function schedule() {
    if (pending) return;
    pending = true;
    requestAnimationFrame(() => {
      pending = false;
      if (draw) { output(draw); draw = ''; }
      byId('counts').textContent = `press ${counts.press} | drag ${counts.drag} | release ${counts.release} | hover ${counts.motion}\nwheel up ${counts.up} | down ${counts.down} | zoom ${zoom}\nnative touch ${counts.native} | untrusted ${counts.untrusted}\nmove prevented ${counts.prevented} | allowed ${counts.allowed}\npage scroll while touching ${counts.scroll}\nreceived cell ${last ? `${last.x},${last.y}` : '-'} | native cell ${nativeCell ? `${nativeCell.x},${nativeCell.y}` : '-'}`;
      byId('log').textContent = log.join('\n') || 'Waiting for input…';
    });
  }

  function receive(data) {
    const text = typeof data === 'string' ? data : new TextDecoder().decode(data);
    for (const match of text.matchAll(/\x1b\[<(\d+);(\d+);(\d+)([Mm])/g)) {
      const code = Number(match[1]);
      const x = Number(match[2]);
      const y = Number(match[3]);
      const release = match[4] === 'm';
      // SGR bits 4/8/16 encode modifiers, not wheel direction or button.
      const base = code & ~(4 | 8 | 16);
      const modifiers = [[4, 'Shift'], [8, 'Meta/Alt'], [16, 'Ctrl']]
        .filter(([bit]) => code & bit).map(([, name]) => name);
      const type = release ? 'release' : base === 32 && !held ? 'motion' : ({ 0: 'press', 1: 'middle press', 2: 'right press', 32: 'drag', 33: 'middle drag', 34: 'right drag', 35: 'motion', 64: 'up', 65: 'down', 66: 'wheel left', 67: 'wheel right' }[base] ?? 'other');
      if (release) held = false;
      else if (base <= 2) held = true;
      if (type in counts) counts[type]++;
      if (type === 'up') zoom++;
      if (type === 'down') zoom--;
      last = { x, y };
      const color = release ? 36 : base >= 64 ? 35 : base === 32 ? 32 : 33;
      if (type !== 'motion') draw += `\x1b[${y};${x}H\x1b[${color}m${release ? 'o' : base >= 64 ? 'Z' : '#'}\x1b[0m`;
      log.push(`ESC[<${code};${x};${y}${match[4]}  ${type}${modifiers.length ? ` [${modifiers.join('+')}]` : ''}`);
      log = log.slice(-5);
    }
    schedule();
  }

  function resetScreen() {
    const cols = terminal.term.cols;
    const rows = terminal.term.rows;
    let screen = '\x1b[?25l\x1b[2J\x1b[H\x1b[90m';
    for (let y = 1; y <= rows; y++) {
      const line = Array.from({ length: cols }, (_, i) => (i + 1) % 10 === 0 ? String(Math.floor((i + 1) / 10) % 10) : (y % 5 === 0 ? '-' : '.')).join('');
      screen += `\x1b[${y};1H${line}`;
    }
    output(screen + '\x1b[0m');
  }

  // Demo policy: keep ghostty's touchend focus listener active, but
  // make its input non-editable until typing is explicitly requested. Do not
  // use disableStdin, which would also disable the gestures under test.
  const hostInput = byId('terminal');
  const textarea = terminal.term.textarea;
  let keyboard = false;
  const setKeyboard = () => {
    hostInput.contentEditable = keyboard ? 'true' : 'false';
    hostInput.inputMode = keyboard ? 'text' : 'none';
    textarea.readOnly = !keyboard;
    textarea.inputMode = keyboard ? 'text' : 'none';
    byId('keyboard').textContent = `Keyboard ${keyboard ? 'ON' : 'OFF'}`;
    if (keyboard) textarea.focus();
    else { textarea.blur(); terminal.blur(); }
  };
  byId('keyboard').onclick = () => { keyboard = !keyboard; setKeyboard(); };
  setKeyboard();
  terminal.connectAdapter({
    boobaRead: () => null,
    boobaWrite: receive,
    boobaResize: () => { if (output) resetScreen(); },
    connect: (onData, onStateChange) => { output = onData; onStateChange('connected', 'Local input sink'); },
    disconnect: () => {},
  });
  const setTracking = () => {
    held = false;
    output(tracking ? '\x1b[?1003h\x1b[?1006h' : '\x1b[?1003l\x1b[?1006l');
    byId('tracking').textContent = `Tracking ${terminal.hasMouseTracking() ? 'ON' : 'OFF'}`;
  };
  byId('tracking').onclick = () => { tracking = !tracking; setTracking(); };
  byId('reset').onclick = () => {
    for (const key of Object.keys(counts)) counts[key] = 0;
    log = []; last = null; nativeCell = null; zoom = 0; draw = ''; held = false;
    resetScreen(); schedule();
  };
  const host = byId('terminal');
  for (const type of ['touchstart', 'touchmove', 'touchend', 'touchcancel']) {
    host.addEventListener(type, event => {
      counts.native++;
      if (!event.isTrusted) counts.untrusted++;
      touches = event.touches.length;
      if (type === 'touchmove') {
        if (event.defaultPrevented) counts.prevented++;
        else counts.allowed++;
      }
      const points = [...event.touches];
      if (!points.length) points.push(...event.changedTouches);
      const canvas = host.querySelector('canvas');
      const metrics = terminal.term.renderer?.getMetrics();
      if (points.length && canvas && metrics) {
        const rect = canvas.getBoundingClientRect();
        const x = points.reduce((sum, p) => sum + p.clientX, 0) / points.length;
        const y = points.reduce((sum, p) => sum + p.clientY, 0) / points.length;
        nativeCell = {
          x: Math.max(1, Math.min(terminal.term.cols, Math.floor((x - rect.left) / metrics.width) + 1)),
          y: Math.max(1, Math.min(terminal.term.rows, Math.floor((y - rect.top) / metrics.height) + 1)),
        };
      }
      schedule();
    }, { passive: true });
  }
  window.addEventListener('scroll', () => { if (touches) { counts.scroll++; schedule(); } }, { passive: true });
  window.addEventListener('pagehide', () => terminal.dispose(), { once: true });
  setTracking(); resetScreen(); schedule();
  byId('status').textContent = `Ready · DPR ${devicePixelRatio} · local JavaScript sink`;

}
