package md.address.events.domain;

import com.fasterxml.jackson.annotation.JsonFormat;
import com.fasterxml.jackson.annotation.JsonProperty;

import java.time.LocalDateTime;
import java.util.List;

public record MessageTranscription(
        String organization,
        @JsonProperty("short_description")
        String shortDescription,
        String event,
        @JsonProperty("event_start")
        @JsonFormat(shape = JsonFormat.Shape.STRING, pattern = "yyyy-MM-dd'T'HH:mm")
        LocalDateTime eventStart,
        @JsonProperty("event_stop")
        @JsonFormat(shape = JsonFormat.Shape.STRING, pattern = "yyyy-MM-dd'T'HH:mm")
        LocalDateTime eventStop,
        List<Address> addresses
) {
}
