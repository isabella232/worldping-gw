package carbon

import (
	"net"

	"github.com/graphite-ng/carbon-relay-ng/input"
	log "github.com/sirupsen/logrus"
)

// handleConn differes from the stock handleConn in 2 ways:
// 1) it logs connection start and end via Info level. whereas carbon-relay-ng would consider this debug level
//    (which itself is largely a consequence of the crng stats library creating a new connection to itself at every flush)
// 2) it helps us do stats our way, with the metrics we care about and the library we use (crng uses a different library)
func handleConn(l *input.Listener, c net.Conn) {
	carbonConnections.Inc()
	log.Infof("%s handler: new tcp connection from %v", l.Handler.Kind(), c.RemoteAddr())

	err := l.Handler.Handle(c)

	carbonConnections.Dec()

	var remoteInfo string

	rAddr := c.RemoteAddr()
	if rAddr != nil {
		remoteInfo = " for " + rAddr.String()
	}
	if err != nil {
		log.Warnf("%s handler%s returned: %s. closing conn", l.Handler.Kind(), remoteInfo, err)
		return
	}
	log.Infof("%s handler%s returned. closing conn", l.Handler.Kind(), remoteInfo)
}
