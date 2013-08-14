(function () {
  "use strict";

  function connect(session) {
    console.log('WAMP client connected');
    document.querySelector('#content').innerHTML = "WAMP client connected to Turnpike server";
  }

  function disconnect(code, reason) {
    console.log('WAMP client disconnected: ' + code + ": " + reason);
    document.querySelector('#content').innerHTML = "WAMP client disconnected: " + reason + " (" + code + ")";
  }
  
  function init() {
    ab.connect("ws://localhost:8080/ws", connect, disconnect);
  }
  window.addEventListener('load', init);
})();
