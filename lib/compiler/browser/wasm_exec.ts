// @ts-nocheck
// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// This file has been modified for use by the TinyGo compiler.
// This file has been further modified for use by Astro.

import { fs, vol } from "memfs";
vol.fromJSON({}, "/");

const enosys = () => {
	const err = new Error("not implemented");
	err.code = "ENOSYS";
	return err;
};

let outputBuf = "";
const process = {
	getuid() { return -1; },
	getgid() { return -1; },
	geteuid() { return -1; },
	getegid() { return -1; },
	getgroups() { throw enosys(); },
	pid: -1,
	ppid: -1,
	umask() { throw enosys(); },
	cwd() { throw enosys(); },
	chdir() { throw enosys(); },
}

// @ts-nocheck
// Shim for using process in browser
// Based off https://github.com/defunctzombie/node-process/blob/master/browser.js
// Based off https://github.com/hardfist/neo-tools/blob/main/packages/bundler/src/shim/process.js

function defaultSetTimout() {
    throw new Error('setTimeout has not been defined');
}
function defaultClearTimeout() {
    throw new Error('clearTimeout has not been defined');
}
var cachedSetTimeout = defaultSetTimout;
var cachedClearTimeout = defaultClearTimeout;
if (typeof globalThis.setTimeout === 'function') {
    cachedSetTimeout = setTimeout;
}
if (typeof globalThis.clearTimeout === 'function') {
    cachedClearTimeout = clearTimeout;
}

function runTimeout(fun) {
    if (cachedSetTimeout === setTimeout) {
        //normal enviroments in sane situations
        return setTimeout(fun, 0);
    }
    // if setTimeout wasn't available but was latter defined
    if ((cachedSetTimeout === defaultSetTimout || !cachedSetTimeout) && setTimeout) {
        cachedSetTimeout = setTimeout;
        return setTimeout(fun, 0);
    }
    try {
        // when when somebody has screwed with setTimeout but no I.E. maddness
        return cachedSetTimeout(fun, 0);
    } catch (e) {
        try {
            // When we are in I.E. but the script has been evaled so I.E. doesn't trust the global object when called normally
            return cachedSetTimeout.call(null, fun, 0);
        } catch (e) {
            // same as above but when it's a version of I.E. that must have the global object for 'this', hopfully our context correct otherwise it will throw a global error
            return cachedSetTimeout.call(this, fun, 0);
        }
    }
}
function runClearTimeout(marker) {
    if (cachedClearTimeout === clearTimeout) {
        //normal enviroments in sane situations
        return clearTimeout(marker);
    }
    // if clearTimeout wasn't available but was latter defined
    if ((cachedClearTimeout === defaultClearTimeout || !cachedClearTimeout) && clearTimeout) {
        cachedClearTimeout = clearTimeout;
        return clearTimeout(marker);
    }
    try {
        // when when somebody has screwed with setTimeout but no I.E. maddness
        return cachedClearTimeout(marker);
    } catch (e) {
        try {
            // When we are in I.E. but the script has been evaled so I.E. doesn't  trust the global object when called normally
            return cachedClearTimeout.call(null, marker);
        } catch (e) {
            // same as above but when it's a version of I.E. that must have the global object for 'this', hopfully our context correct otherwise it will throw a global error.
            // Some versions of I.E. have different rules for clearTimeout vs setTimeout
            return cachedClearTimeout.call(this, marker);
        }
    }
}
var queue = [];
var draining = false;
var currentQueue;
var queueIndex = -1;

function cleanUpNextTick() {
    if (!draining || !currentQueue) {
        return;
    }
    draining = false;
    if (currentQueue.length) {
        queue = currentQueue.concat(queue);
    } else {
        queueIndex = -1;
    }
    if (queue.length) {
        drainQueue();
    }
}

function drainQueue() {
    if (draining) {
        return;
    }
    var timeout = runTimeout(cleanUpNextTick);
    draining = true;

    var len = queue.length;
    while (len) {
        currentQueue = queue;
        queue = [];
        while (++queueIndex < len) {
            if (currentQueue) {
                currentQueue[queueIndex].run();
            }
        }
        queueIndex = -1;
        len = queue.length;
    }
    currentQueue = null;
    draining = false;
    runClearTimeout(timeout);
}
function nextTick(fun) {
    var args = new Array(arguments.length - 1);
    if (arguments.length > 1) {
        for (var i = 1; i < arguments.length; i++) {
            args[i - 1] = arguments[i];
        }
    }
    queue.push(new Item(fun, args));
    if (queue.length === 1 && !draining) {
        runTimeout(drainQueue);
    }
}
// v8 likes predictible objects
function Item(fun, array) {
    this.fun = fun;
    this.array = array;
}
Item.prototype.run = function () {
    this.fun.apply(null, this.array);
};
var title = 'browser';
var platform = 'browser';
var browser = true;
var env = {};
var argv = [];
var version = ''; // empty string to avoid regexp issues
var versions = {};
var release = {};
var config = {};

function noop() { }

var on = noop;
var addListener = noop;
var once = noop;
var off = noop;
var removeListener = noop;
var removeAllListeners = noop;
var emit = noop;

function binding(name) {
    throw new Error('process.binding is not supported');
}

function cwd() {
    return '/';
}
function chdir(dir) {
    throw new Error('process.chdir is not supported');
}
function umask() {
    return 0;
}

// from https://github.com/kumavis/browser-process-hrtime/blob/master/index.js
var performance = globalThis.performance || {};
var performanceNow =
    performance.now ||
    performance.mozNow ||
    performance.msNow ||
    performance.oNow ||
    performance.webkitNow ||
    function () {
        return new Date().getTime();
    };

// generate timestamp or delta
// see http://nodejs.org/api/process.html#process_process_hrtime
function hrtime(previousTimestamp) {
    var clocktime = performanceNow.call(performance) * 1e-3;
    var seconds = Math.floor(clocktime);
    var nanoseconds = Math.floor((clocktime % 1) * 1e9);
    if (previousTimestamp) {
        seconds = seconds - previousTimestamp[0];
        nanoseconds = nanoseconds - previousTimestamp[1];
        if (nanoseconds < 0) {
            seconds--;
            nanoseconds += 1e9;
        }
    }
    return [seconds, nanoseconds];
}

var startTime = new Date();
function uptime() {
    var currentTime = new Date();
    var dif = currentTime - startTime;
    return dif / 1000;
}

var browser$1 = {
    nextTick: nextTick,
    title: title,
    browser: browser,
    env: env,
    argv: argv,
    version: version,
    versions: versions,
    on: on,
    addListener: addListener,
    once: once,
    off: off,
    removeListener: removeListener,
    removeAllListeners: removeAllListeners,
    emit: emit,
    binding: binding,
    cwd: cwd,
    chdir: chdir,
    umask: umask,
    hrtime: hrtime,
    platform: platform,
    release: release,
    config: config,
    uptime: uptime,
};

export default browser$1;
delete versions['node']; // hack
Object.assign(process, {
    addListener,
    argv,
    binding,
    browser,
    chdir,
    config,
    cwd,
    emit,
    env,
    hrtime,
    nextTick,
    off,
    on,
    once,
    platform,
    release,
    removeAllListeners,
    removeListener,
    title,
    umask,
    uptime,
    version,
    versions,
}, browser$1);

Object.defineProperties(globalThis, {
  fs: {
    value: fs,
    enumerable: true,
  },
  process: {
    value: process,
    enumerable: true
  }
});

const encoder = new TextEncoder("utf-8");
const decoder = new TextDecoder("utf-8");
var logLine = [];

export default class Go {
  importObject: Record<string, any>;

  constructor() {
    this._callbackTimeouts = new Map();
    this._nextCallbackTimeoutID = 1;

    const mem = () => {
      // The buffer may change when requesting more memory.
      return new DataView(this._inst.exports.memory.buffer);
    };

    const setInt64 = (addr, v) => {
      mem().setUint32(addr + 0, v, true);
      mem().setUint32(addr + 4, Math.floor(v / 4294967296), true);
    };

    const getInt64 = (addr) => {
      const low = mem().getUint32(addr + 0, true);
      const high = mem().getInt32(addr + 4, true);
      return low + high * 4294967296;
    };

    const loadValue = (addr) => {
      const f = mem().getFloat64(addr, true);
      if (f === 0) {
        return undefined;
      }
      if (!isNaN(f)) {
        return f;
      }

      const id = mem().getUint32(addr, true);
      return this._values[id];
    };

    const storeValue = (addr, v) => {
      const nanHead = 0x7ff80000;

      if (typeof v === "number") {
        if (isNaN(v)) {
          mem().setUint32(addr + 4, nanHead, true);
          mem().setUint32(addr, 0, true);
          return;
        }
        if (v === 0) {
          mem().setUint32(addr + 4, nanHead, true);
          mem().setUint32(addr, 1, true);
          return;
        }
        mem().setFloat64(addr, v, true);
        return;
      }

      switch (v) {
        case undefined:
          mem().setFloat64(addr, 0, true);
          return;
        case null:
          mem().setUint32(addr + 4, nanHead, true);
          mem().setUint32(addr, 2, true);
          return;
        case true:
          mem().setUint32(addr + 4, nanHead, true);
          mem().setUint32(addr, 3, true);
          return;
        case false:
          mem().setUint32(addr + 4, nanHead, true);
          mem().setUint32(addr, 4, true);
          return;
      }

      let id = this._ids.get(v);
      if (id === undefined) {
        id = this._idPool.pop();
        if (id === undefined) {
          id = this._values.length;
        }
        this._values[id] = v;
        this._goRefCounts[id] = 0;
        this._ids.set(v, id);
      }
      this._goRefCounts[id]++;
      let typeFlag = 1;
      switch (typeof v) {
        case "string":
          typeFlag = 2;
          break;
        case "symbol":
          typeFlag = 3;
          break;
        case "function":
          typeFlag = 4;
          break;
      }
      mem().setUint32(addr + 4, nanHead | typeFlag, true);
      mem().setUint32(addr, id, true);
    };

    const loadSlice = (array, len, cap) => {
      return new Uint8Array(this._inst.exports.memory.buffer, array, len);
    };

    const loadSliceOfValues = (array, len, cap) => {
      const a = new Array(len);
      for (let i = 0; i < len; i++) {
        a[i] = loadValue(array + i * 8);
      }
      return a;
    };

    const loadString = (ptr, len) => {
      return decoder.decode(
        new DataView(this._inst.exports.memory.buffer, ptr, len)
      );
    };

    const timeOrigin = Date.now() - performance.now();
    this.importObject = {
      wasi_snapshot_preview1: {
        // https://github.com/WebAssembly/WASI/blob/main/phases/snapshot/docs.md#fd_write
        fd_write: function (fd, iovs_ptr, iovs_len, nwritten_ptr) {
          let nwritten = 0;
          if (fd == 1) {
            for (let iovs_i = 0; iovs_i < iovs_len; iovs_i++) {
              let iov_ptr = iovs_ptr + iovs_i * 8; // assuming wasm32
              let ptr = mem().getUint32(iov_ptr + 0, true);
              let len = mem().getUint32(iov_ptr + 4, true);
              for (let i = 0; i < len; i++) {
                let c = mem().getUint8(ptr + i);
                if (c == 13) {
                  // CR
                  // ignore
                } else if (c == 10) {
                  // LF
                  // write line
                  let line = decoder.decode(new Uint8Array(logLine));
                  logLine = [];
                  console.log(line);
                } else {
                  logLine.push(c);
                }
              }
            }
          } else {
            console.error("invalid file descriptor:", fd);
          }
          mem().setUint32(nwritten_ptr, nwritten, true);
          return 0;
        },
        proc_exit: (code) => {
          if (globalThis.process) {
            // Node.js
            process.exit(code);
          } else {
            // Can't exit in a browser.
            throw "trying to exit with code " + code;
          }
        },
      },
      env: {
        // func ticks() float64
        "runtime.ticks": () => {
          return timeOrigin + performance.now();
        },

        // func sleepTicks(timeout float64)
        "runtime.sleepTicks": (timeout) => {
          // Do not sleep, only reactivate scheduler after the given timeout.
          setTimeout(this._inst.exports.go_scheduler, timeout);
        },

        // func finalizeRef(v ref)
        "syscall/js.finalizeRef": (v_addr) => {
          // Note: TinyGo does not support finalizers so this is only called
          // for one specific case, by js.go:jsString.
          const id = mem().getUint32(v_addr, true);
          this._goRefCounts[id]--;
          if (this._goRefCounts[id] === 0) {
            const v = this._values[id];
            this._values[id] = null;
            this._ids.delete(v);
            this._idPool.push(id);
          }
        },

        // func stringVal(value string) ref
        "syscall/js.stringVal": (ret_ptr, value_ptr, value_len) => {
          const s = loadString(value_ptr, value_len);
          storeValue(ret_ptr, s);
        },

        // func valueGet(v ref, p string) ref
        "syscall/js.valueGet": (retval, v_addr, p_ptr, p_len) => {
          let prop = loadString(p_ptr, p_len);
          let value = loadValue(v_addr);
          let result = Reflect.get(value, prop);
          storeValue(retval, result);
        },

        // func valueSet(v ref, p string, x ref)
        "syscall/js.valueSet": (v_addr, p_ptr, p_len, x_addr) => {
          const v = loadValue(v_addr);
          const p = loadString(p_ptr, p_len);
          const x = loadValue(x_addr);
          Reflect.set(v, p, x);
        },

        // func valueDelete(v ref, p string)
        "syscall/js.valueDelete": (v_addr, p_ptr, p_len) => {
          const v = loadValue(v_addr);
          const p = loadString(p_ptr, p_len);
          Reflect.deleteProperty(v, p);
        },

        // func valueIndex(v ref, i int) ref
        "syscall/js.valueIndex": (ret_addr, v_addr, i) => {
          storeValue(ret_addr, Reflect.get(loadValue(v_addr), i));
        },

        // valueSetIndex(v ref, i int, x ref)
        "syscall/js.valueSetIndex": (v_addr, i, x_addr) => {
          Reflect.set(loadValue(v_addr), i, loadValue(x_addr));
        },

        // func valueCall(v ref, m string, args []ref) (ref, bool)
        "syscall/js.valueCall": (
          ret_addr,
          v_addr,
          m_ptr,
          m_len,
          args_ptr,
          args_len,
          args_cap
        ) => {
          const v = loadValue(v_addr);
          const name = loadString(m_ptr, m_len);
          const args = loadSliceOfValues(args_ptr, args_len, args_cap);
          try {
            const m = Reflect.get(v, name);
            storeValue(ret_addr, Reflect.apply(m, v, args));
            mem().setUint8(ret_addr + 8, 1);
          } catch (err) {
            storeValue(ret_addr, err);
            mem().setUint8(ret_addr + 8, 0);
          }
        },

        // func valueInvoke(v ref, args []ref) (ref, bool)
        "syscall/js.valueInvoke": (
          ret_addr,
          v_addr,
          args_ptr,
          args_len,
          args_cap
        ) => {
          try {
            const v = loadValue(v_addr);
            const args = loadSliceOfValues(args_ptr, args_len, args_cap);
            storeValue(ret_addr, Reflect.apply(v, undefined, args));
            mem().setUint8(ret_addr + 8, 1);
          } catch (err) {
            storeValue(ret_addr, err);
            mem().setUint8(ret_addr + 8, 0);
          }
        },

        // func valueNew(v ref, args []ref) (ref, bool)
        "syscall/js.valueNew": (
          ret_addr,
          v_addr,
          args_ptr,
          args_len,
          args_cap
        ) => {
          const v = loadValue(v_addr);
          const args = loadSliceOfValues(args_ptr, args_len, args_cap);
          try {
            storeValue(ret_addr, Reflect.construct(v, args));
            mem().setUint8(ret_addr + 8, 1);
          } catch (err) {
            storeValue(ret_addr, err);
            mem().setUint8(ret_addr + 8, 0);
          }
        },

        // func valueLength(v ref) int
        "syscall/js.valueLength": (v_addr) => {
          return loadValue(v_addr).length;
        },

        // valuePrepareString(v ref) (ref, int)
        "syscall/js.valuePrepareString": (ret_addr, v_addr) => {
          const s = String(loadValue(v_addr));
          const str = encoder.encode(s);
          storeValue(ret_addr, str);
          setInt64(ret_addr + 8, str.length);
        },

        // valueLoadString(v ref, b []byte)
        "syscall/js.valueLoadString": (
          v_addr,
          slice_ptr,
          slice_len,
          slice_cap
        ) => {
          const str = loadValue(v_addr);
          loadSlice(slice_ptr, slice_len, slice_cap).set(str);
        },

        // func valueInstanceOf(v ref, t ref) bool
        "syscall/js.valueInstanceOf": (v_addr, t_addr) => {
          return loadValue(v_addr) instanceof loadValue(t_addr);
        },

        // func copyBytesToGo(dst []byte, src ref) (int, bool)
        "syscall/js.copyBytesToGo": (
          ret_addr,
          dest_addr,
          dest_len,
          dest_cap,
          source_addr
        ) => {
          let num_bytes_copied_addr = ret_addr;
          let returned_status_addr = ret_addr + 4; // Address of returned boolean status variable

          const dst = loadSlice(dest_addr, dest_len);
          const src = loadValue(source_addr);
          if (!(src instanceof Uint8Array)) {
            mem().setUint8(returned_status_addr, 0); // Return "not ok" status
            return;
          }
          const toCopy = src.subarray(0, dst.length);
          dst.set(toCopy);
          setInt64(num_bytes_copied_addr, toCopy.length);
          mem().setUint8(returned_status_addr, 1); // Return "ok" status
        },

        // copyBytesToJS(dst ref, src []byte) (int, bool)
        // Originally copied from upstream Go project, then modified:
        //   https://github.com/golang/go/blob/3f995c3f3b43033013013e6c7ccc93a9b1411ca9/misc/wasm/wasm_exec.js#L404-L416
        "syscall/js.copyBytesToJS": (
          ret_addr,
          dest_addr,
          source_addr,
          source_len,
          source_cap
        ) => {
          let num_bytes_copied_addr = ret_addr;
          let returned_status_addr = ret_addr + 4; // Address of returned boolean status variable

          const dst = loadValue(dest_addr);
          const src = loadSlice(source_addr, source_len);
          if (!(dst instanceof Uint8Array)) {
            mem().setUint8(returned_status_addr, 0); // Return "not ok" status
            return;
          }
          const toCopy = src.subarray(0, dst.length);
          dst.set(toCopy);
          setInt64(num_bytes_copied_addr, toCopy.length);
          mem().setUint8(returned_status_addr, 1); // Return "ok" status
        },
      },
    };
  }

  async run(instance) {
    this._inst = instance;
    this._values = [
      // JS values that Go currently has references to, indexed by reference id
      NaN,
      0,
      null,
      true,
      false,
      globalThis,
      this,
    ];
    this._goRefCounts = []; // number of references that Go has to a JS value, indexed by reference id
    this._ids = new Map(); // mapping from JS values to reference ids
    this._idPool = []; // unused ids that have been garbage collected
    this.exited = false; // whether the Go program has exited

    const mem = new DataView(this._inst.exports.memory.buffer);

    while (true) {
      const callbackPromise = new Promise((resolve) => {
        this._resolveCallbackPromise = () => {
          if (this.exited) {
            throw new Error("bad callback: Go program has already exited");
          }
          setTimeout(resolve, 0); // make sure it is asynchronous
        };
      });
      this._inst.exports._start();
      if (this.exited) {
        break;
      }
      await callbackPromise;
    }
  }

  private _resume() {
    if (this.exited) {
      throw new Error("Go program has already exited");
    }
    this._inst.exports.resume();
    if (this.exited) {
      this._resolveExitPromise();
    }
  }

  private _makeFuncWrapper(id) {
    const go = this;
    return function () {
      const event = { id: id, this: this, args: arguments };
      go._pendingEvent = event;
      go._resume();
      return event.result;
    };
  }
}
