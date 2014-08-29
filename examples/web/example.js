(function () {
	"use strict";

	var conn,
		session,
		stage,
		layer,
		colo,
		channel = 'my.turnpike.chat';

	function drawCircle(x, y, color) {
		var circ = new Kinetic.Circle({
			x: x,
			y: y,
			radius: 0,
			fill: color,
		});

		layer.add(circ);
		circ.tween = new Kinetic.Tween({
			node: circ,
			radius: 75,
			opacity: 0,
			easing: Kinetic.Easings.EaseIn,
			duration: 1,
			onFinish: function () {
				circ.remove();
			}
		});
		circ.tween.play();
	}

	function randColor() {
		return 'rgb(' + Math.random() + ',' + Math.random() + ',' + Math.random() + ')';
	}

	function initKinectic() {
		colo = randColor();

		stage = new Kinetic.Stage({
			container: 'container',
			width: window.innerWidth,
			height: window.innerHeight,
		});

		layer = new Kinetic.Layer();
		stage.add(layer);

		// add click and tap handlers
		stage.on('contentClick', function () {
			var mousePos = stage.getPointerPosition();
			session.publish(channel, [mousePos.x, mousePos.y, colo]);
		});
	}

	function initAutobahn() {
		conn = new autobahn.Connection({
			url: location.href.replace(/^http/, 'ws') + 'ws',
			realm: 'turnpike.chat.realm',
		});

		conn.onopen = function (sess) {
			function onevent(args) {
				drawCircle.apply(null, args);
			}

			session = sess;
			sess.subscribe(channel, onevent);
		};

		conn.open();
	}

	function init() {
		initAutobahn();
		initKinectic();
	}

	window.addEventListener('load', init);
}());
