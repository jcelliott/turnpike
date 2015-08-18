#!/usr/bin/env python2
import sys
import trollius as asyncio
from trollius import From
from autobahn.asyncio.wamp import ApplicationSession, ApplicationRunner

class MyComponent(ApplicationSession):
    @asyncio.coroutine
    def onJoin(self, details):
        print("session joined")
        @asyncio.coroutine
        def hello(name):
            print("Hello, {}!".format(name))
        registration = yield From(self.register(hello, u'turnpike.examples.hello'))
        print("registered procedure")
        yield From(self.call(u'turnpike.examples.hello', "Turnpike"))
        print("called procedure")
        yield From(registration.unregister())
        print("unregistered procedure")
        yield From(self.leave())

    def onLeave(self, details):
        print("session closed: {}".format(details))
        self.disconnect()

    def onDisconnect(self):
        sys.exit("client disconnected")


if __name__ == '__main__':
    runner = ApplicationRunner(url=u"ws://localhost:8000", realm=u"turnpike.examples", debug_wamp=True)
    runner.run(MyComponent)
