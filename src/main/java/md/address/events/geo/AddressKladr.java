package md.address.events.geo;

import com.fasterxml.jackson.annotation.JsonProperty;

public record AddressKladr(
    String country,
    @JsonProperty("full_address")
    String fullAddress,
    String house,
    String kladr,
    KladrEntity region,
    KladrEntity city,
    KladrEntity street
) {
}
