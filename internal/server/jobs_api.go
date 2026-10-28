package server

// maxReading is the number of reading slots: the generation starts that read
// and validate a body at once. A body is up to maxBodyBytes and is held with
// its parse while it is checked; two match the legacy endpoint, which reads a
// body only while it holds one of the two generation slots.
const maxReading = 2
