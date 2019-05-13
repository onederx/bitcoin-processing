package events

func (e *eventBroker) sendWSNotifications() {
	seq, err := e.storage.GetLastWSSentSeq()
	if err != nil {
		panic(err) // TODO: retry
	}
	events, err := e.GetEventsFromSeq(seq + 1)
	if err != nil {
		panic(err) // TODO: retry
	}
	if len(events) == 0 {
		// TODO: logging
		return
	}
	lastSeq := -1
	for _, event := range events {
		// no error checking here: client is responsible for event loss
		e.eventBroadcaster.Broadcast(event)
		lastSeq = event.Seq
	}
	if lastSeq == -1 {
		// should not happen because we checked for len == 0 earlier
		panic("events: last seq was not set in broadcasting WS events")
	}
	err = e.storage.StoreLastWSSentSeq(lastSeq)
	if err != nil {
		// TODO: retry to store sent seq and die if that does not work
		// with a message that a client should update seqnum manually
		// What if we crash here due to, say, SIGKILL ? Seems that client
		// should handle event repeats anyway
		// We could maintain some kind of 'dirty' flag in DB that says
		// that an attempt to send event via HTTP callback was made, but
		// we are not sure if it succeeded - so a user should manually
		// settle this, adjust seqnum and clear 'dirty flag'
		// a transaction can be used here to clear 'dirty' flag and adjust
		// seqnum
		panic(err)
	}
}
