export class TerminalSequenceObserver {
    constructor(callbacks, maxOSCBytes = 4096) {
        this.callbacks = callbacks;
        this.maxOSCBytes = maxOSCBytes;
        this.state = 'ground';
        this.escaped = false;
        this.body = '';
        this.discarded = false;
        this.alternate = false;
        this.offset = 0;
    }
    reset() {
        this.state = 'ground';
        this.escaped = false;
        this.body = '';
        this.discarded = false;
        this.alternate = false;
    }
    scan(data) {
        for (let i = 0; i < data.length; i++) {
            this.offset = i + 1;
            this.byte(typeof data === 'string' ? data.charCodeAt(i) : data[i]);
        }
    }
    start(state) {
        this.state = state;
        this.body = '';
        this.escaped = false;
        this.discarded = false;
    }
    byte(c) {
        // CAN/SUB cancel an unfinished sequence without applying any of it.
        if (c === 0x18 || c === 0x1a) {
            this.start('ground');
            return;
        }
        if (this.state === 'osc' || this.state === 'string') {
            if (this.escaped) {
                this.escaped = false;
                if (c === 0x5c) {
                    this.finishString();
                    return;
                }
                // Keep other control strings opaque through ST. In OSC, ESC
                // cancels the old string and starts a new escape sequence.
                if (this.state === 'osc') {
                    this.start('escape');
                    this.byte(c);
                    return;
                }
            }
            if (c === 0x1b) {
                this.escaped = true;
                return;
            }
            if (this.state === 'string')
                return;
            if (c === 7) {
                this.finishString();
                return;
            }
            if (c < 0x20 || c > 0x7e)
                this.discarded = true;
            if (!this.discarded) {
                if (this.body.length >= this.maxOSCBytes - 4) {
                    this.discarded = true;
                    this.body = '';
                }
                else
                    this.body += String.fromCharCode(c);
            }
            return;
        }
        if (c === 0x1b) {
            this.start('escape');
            return;
        }
        if (this.state === 'escape') {
            if (c === 0x5d)
                this.start('osc');
            else if (c === 0x5b)
                this.start('csi');
            else if ([0x50, 0x58, 0x5e, 0x5f].includes(c))
                this.start('string');
            else {
                this.start('ground');
                if (c === 0x63) {
                    this.alternate = false;
                    this.callbacks.lifecycle('reset', this.offset);
                }
            }
            return;
        }
        if (this.state === 'csi') {
            if (c >= 0x40 && c <= 0x7e) {
                const body = this.body;
                const valid = !this.discarded;
                this.start('ground');
                if (valid && (c === 0x68 || c === 0x6c) && /^\?\d+(;\d+)*$/.test(body) &&
                    body.slice(1).split(';').some(mode => ['47', '1047', '1049'].includes(mode))) {
                    const alternate = c === 0x68;
                    if (alternate !== this.alternate) {
                        this.alternate = alternate;
                        this.callbacks.lifecycle('screen', this.offset);
                    }
                }
            }
            else if (c < 0x20 || c > 0x3f || this.body.length >= 64) {
                this.discarded = true;
                this.body = '';
            }
            else if (!this.discarded)
                this.body += String.fromCharCode(c);
        }
    }
    finishString() {
        const body = this.body;
        const apply = this.state === 'osc' && !this.discarded;
        this.start('ground');
        if (!apply)
            return;
        if (/^133;A(?:;|$)/.test(body))
            this.callbacks.lifecycle('prompt', this.offset);
        this.callbacks.osc(body);
    }
}
