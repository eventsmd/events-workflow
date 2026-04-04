package md.address.events.geo;

import md.address.events.ai.AddressPicker;
import md.address.events.domain.*;
import md.address.events.persistence.AddressEntity;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;

import java.math.BigInteger;
import java.time.LocalDateTime;
import java.util.List;

import static org.junit.jupiter.api.Assertions.*;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.*;

@ExtendWith(MockitoExtension.class)
class AddressAdapterTest {

    @Mock private AddressApi addressApi;
    @Mock private AddressPicker addressPicker;

    private AddressAdapter adapter;

    @BeforeEach
    void setUp() {
        adapter = new AddressAdapter(addressApi, addressPicker);
    }

    @Test
    void enrichShouldSetFieldsFromSingleResult() {
        var entity = new AddressEntity();
        entity.setCityOriginal("Кишинёв");
        entity.setStreetOriginal("Пушкина");

        var region = new KladrEntity("5100000000000", "Молдова", "Молдова", null, null);
        var city = new KladrEntity("5100000100000", "Кишинёв", "г Кишинёв", "г", "город");
        var street = new KladrEntity("5100000100078", "Пушкина", "ул Пушкина", "ул", "улица");
        var kladr = new AddressKladr("MD", "Кишинёв, ул Пушкина", null, "5100000100078", region, city, street);

        when(addressApi.find("Кишинёв Пушкина")).thenReturn(List.of(kladr));

        adapter.enrich(createParsedMessage(), entity);

        assertEquals("5100000000000", entity.getRegionKladr());
        assertEquals("Молдова", entity.getRegionName());
        assertEquals("5100000100000", entity.getCityKladr());
        assertEquals("Кишинёв", entity.getCityName());
        assertEquals("5100000100078", entity.getStreetKladr());
        assertEquals("Пушкина", entity.getStreetName());
        assertEquals("ул", entity.getStreetType());
    }

    @Test
    void enrichShouldDelegateToPickerWhenMultipleResults() {
        var entity = new AddressEntity();
        entity.setCityOriginal("Кишинёв");
        entity.setStreetOriginal("Пушкина");

        var street1 = new KladrEntity("510001", "Пушкина", "ул Пушкина", "ул", "улица");
        var street2 = new KladrEntity("510002", "Пушкинская", "ул Пушкинская", "ул", "улица");
        var kladr1 = new AddressKladr("MD", "addr1", null, "1", null, null, street1);
        var kladr2 = new AddressKladr("MD", "addr2", null, "2", null, null, street2);

        when(addressApi.find("Кишинёв Пушкина")).thenReturn(List.of(kladr1, kladr2));
        when(addressPicker.pickAddress(any(), eq(entity), any())).thenReturn(kladr1);

        adapter.enrich(createParsedMessage(), entity);

        verify(addressPicker).pickAddress(any(), eq(entity), eq(List.of(kladr1, kladr2)));
        assertEquals("510001", entity.getStreetKladr());
    }

    @Test
    void enrichShouldDoNothingWhenApiReturnsNull() {
        var entity = new AddressEntity();
        entity.setCityOriginal("Город");
        entity.setStreetOriginal("Улица");

        when(addressApi.find("Город Улица")).thenReturn(null);

        adapter.enrich(createParsedMessage(), entity);

        assertNull(entity.getStreetKladr());
        verify(addressPicker, never()).pickAddress(any(), any(), any());
    }

    @Test
    void enrichShouldDoNothingWhenApiReturnsEmptyList() {
        var entity = new AddressEntity();
        entity.setCityOriginal("Город");
        entity.setStreetOriginal("Улица");

        when(addressApi.find("Город Улица")).thenReturn(List.of());

        adapter.enrich(createParsedMessage(), entity);

        assertNull(entity.getStreetKladr());
    }

    @Test
    void enrichShouldHandlePartialKladrData() {
        var entity = new AddressEntity();
        entity.setCityOriginal("Бэлць");
        entity.setStreetOriginal("Ленина");

        // Only street info, no region or city
        var street = new KladrEntity("510001", "Ленина", "ул Ленина", "ул", "улица");
        var kladr = new AddressKladr("MD", "addr", null, "1", null, null, street);

        when(addressApi.find("Бэлць Ленина")).thenReturn(List.of(kladr));

        adapter.enrich(createParsedMessage(), entity);

        assertNull(entity.getRegionKladr());
        assertNull(entity.getCityKladr());
        assertEquals("510001", entity.getStreetKladr());
    }

    private ParsedMessage createParsedMessage() {
        var message = new TelegramMessage(
                BigInteger.ONE, BigInteger.TWO,
                new User(BigInteger.ONE, "User"),
                "Test text", LocalDateTime.now(),
                null, null, null
        );
        return new ParsedMessage(message, null);
    }
}
