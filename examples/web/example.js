(function () {
	"use strict";

	var conn,
		session,
		renderer,
		stage,
		colo,
		channel = 'my.turnpike.chat';

	function drawCircle(x, y, color) {
		var g = new PIXI.Graphics(),
			fields = {
				radius: 0,
				opacity: 1,
			};

		console.log("X:", x, "Y:", y);
		stage.addChild(g);

		function handleChange() {
			// console.log("Radius:", fields.radius, "Opacity:", fields.opacity);
			g.clear();
			g.beginFill(color, fields.opacity);
			g.drawCircle(x, y, fields.radius);
			g.endFill();
		}

		function remove() {
			stage.removeChild(g);
		}

		createjs.Tween.get(fields).to({radius: 100, opacity: 0}, 1500).addEventListener('change', handleChange).call(remove);
	}

	function randColor() {
		return (Math.random()*255 << 16) | (Math.random()*255 << 8) | (Math.random()*255);
	}

	function draw() {
		renderer.render(stage);
		window.requestAnimationFrame(draw);
	}

	function initDrawing() {
		colo = randColor();

		stage = new PIXI.Container();
		stage.interactive = true;
		stage.buttonMode = true;
		renderer = new PIXI.autoDetectRenderer(window.innerWidth, window.innerHeight);
		document.body.appendChild(renderer.view);

		stage.hitArea = new PIXI.Rectangle(0, 0, window.innerWidth, window.innerHeight);

		function eventHandler(e) {
			var mousePos = e.data.global;
			session.publish(channel, [mousePos.x, mousePos.y, colo]);
			drawCircle(mousePos.x, mousePos.y, colo)
		}

		// add click and tap handlers
		stage.on('mousedown', eventHandler);
		stage.on('touchstart', eventHandler);

		window.requestAnimationFrame(draw);
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
		initDrawing();
	}

	window.addEventListener('load', init);
}());
