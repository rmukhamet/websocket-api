FROM scratch

COPY websocket_service /

EXPOSE 7070

ENTRYPOINT ["/websocket_service"]
