package md.address.events.geo;

import md.address.events.ai.AddressPicker;
import md.address.events.domain.ParsedMessage;
import md.address.events.persistence.AddressEntity;
import org.springframework.stereotype.Component;

import java.util.List;

import static java.util.Optional.ofNullable;

@Component
public class AddressAdapter {

    private final AddressApi addressApi;
    private final AddressPicker addressPicker;

    public AddressAdapter(AddressApi addressApi, AddressPicker addressPicker) {
        this.addressApi = addressApi;
        this.addressPicker = addressPicker;
    }

    public void enrich(ParsedMessage message, AddressEntity address) {

        List<AddressKladr> addresses =
                addressApi.find("%s %s".formatted(address.getCityOriginal(), address.getStreetOriginal()));

        if(addresses != null && !addresses.isEmpty()) {

            if (addresses.size() == 1) {
                enrich(address, addresses.getFirst());
            } else {
                enrich(
                        address,
                        addressPicker.pickAddress(message, address, addresses)
                );
            }
        }
    }

    private void enrich(AddressEntity entity, AddressKladr addressKladr) {

        ofNullable(addressKladr.region())
                .ifPresent(region -> {
                    entity.setRegionKladr(region.kladr());
                    entity.setRegionName(region.name());
                });
        ofNullable(addressKladr.city())
                .ifPresent(city -> {
                    entity.setCityKladr(city.kladr());
                    entity.setCityName(city.name());
                });
        ofNullable(addressKladr.street())
                .ifPresent(street -> {
                    entity.setStreetType(street.type());
                    entity.setStreetKladr(street.kladr());
                    entity.setStreetName(street.name());
                });
    }
}
