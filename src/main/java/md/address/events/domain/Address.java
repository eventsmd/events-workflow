package md.address.events.domain;

import com.fasterxml.jackson.annotation.JsonProperty;

import java.util.UUID;

public record Address (
    UUID id,
    String city,
    String street,
    @JsonProperty("street_type")
    String streetType,
    House house
){
    public Address {
        if(id == null) id = UUID.randomUUID();
    }
}
