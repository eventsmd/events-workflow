package md.address.events.persistence;

import jakarta.persistence.Column;
import jakarta.persistence.Entity;
import jakarta.persistence.Id;
import jakarta.persistence.Table;
import md.address.events.domain.Address;
import md.address.events.domain.MessageReference;

import java.math.BigInteger;
import java.util.UUID;

@Entity
@Table(name = "incident_address")
public class AddressEntity {

    @Id
    private UUID id;

    @Column(name = "message_id", nullable = false)
    private BigInteger messageId;
    @Column(name = "chat_id", nullable = false)
    private BigInteger chatId;
    @Column(name = "city_original")
    private String cityOriginal;
    @Column(name = "street_original")
    private String streetOriginal;
    @Column(name = "street_type_original")
    private String streetTypeOriginal;
    @Column(name = "house_numbers")
    private String houseNumbers;
    @Column(name = "house_ranges")
    private String houseRanges;
    @Column(name = "region_name")
    private String regionName;
    @Column(name = "region_kladr")
    private String regionKladr;
    @Column(name = "region_type")
    private String regionType;
    @Column(name = "city_name")
    private String cityName;
    @Column(name = "city_kladr")
    private String cityKladr;
    @Column(name = "city_type")
    private String cityType;
    @Column(name = "street_name")
    private String streetName;
    @Column(name = "street_kladr")
    private String streetKladr;
    @Column(name = "street_type")
    private String streetType;

    public static AddressEntity from(Address address, MessageReference messageReference) {
        var entity = new AddressEntity();
        entity.setId(address.id());
        entity.setCityOriginal(address.city());
        entity.setStreetOriginal(address.street());
        entity.setStreetTypeOriginal(address.streetType());
        entity.setMessageId(messageReference.id());
        entity.setChatId(messageReference.chatId());
        if (address.house() != null) {
            if (address.house().numbers() != null && !address.house().numbers().isEmpty()) {
                entity.setHouseNumbers(String.join(",", address.house().numbers()));
            }
            if (address.house().ranges() != null && !address.house().ranges().isEmpty()) {
                entity.setHouseRanges(address.house().ranges().stream()
                        .map(range -> String.join("-", range))
                        .collect(java.util.stream.Collectors.joining(";")));
            }
        }
        return entity;
    }

    public String formatHouses() {
        var parts = new java.util.ArrayList<String>();
        if (houseNumbers != null && !houseNumbers.isBlank()) {
            parts.addAll(java.util.List.of(houseNumbers.split(",")));
        }
        if (houseRanges != null && !houseRanges.isBlank()) {
            parts.addAll(java.util.List.of(houseRanges.split(";")));
        }
        if (parts.isEmpty()) return "";
        return "д. " + String.join(", ", parts);
    }

    public UUID getId() {
        return id;
    }

    public void setId(UUID id) {
        this.id = id;
    }

    public BigInteger getMessageId() {
        return messageId;
    }

    public void setMessageId(BigInteger messageId) {
        this.messageId = messageId;
    }

    public BigInteger getChatId() {
        return chatId;
    }

    public void setChatId(BigInteger chatId) {
        this.chatId = chatId;
    }

    public String getCityOriginal() {
        return cityOriginal;
    }

    public void setCityOriginal(String cityOriginal) {
        this.cityOriginal = cityOriginal;
    }

    public String getStreetOriginal() {
        return streetOriginal;
    }

    public void setStreetOriginal(String streetOriginal) {
        this.streetOriginal = streetOriginal;
    }

    public String getStreetTypeOriginal() {
        return streetTypeOriginal;
    }

    public void setStreetTypeOriginal(String streetTypeOriginal) {
        this.streetTypeOriginal = streetTypeOriginal;
    }

    public String getHouseNumbers() {
        return houseNumbers;
    }

    public void setHouseNumbers(String houseNumbers) {
        this.houseNumbers = houseNumbers;
    }

    public String getHouseRanges() {
        return houseRanges;
    }

    public void setHouseRanges(String houseRanges) {
        this.houseRanges = houseRanges;
    }

    public String getRegionName() {
        return regionName;
    }

    public void setRegionName(String regionName) {
        this.regionName = regionName;
    }

    public String getRegionKladr() {
        return regionKladr;
    }

    public void setRegionKladr(String regionKladr) {
        this.regionKladr = regionKladr;
    }

    public String getRegionType() {
        return regionType;
    }

    public void setRegionType(String regionType) {
        this.regionType = regionType;
    }

    public String getCityName() {
        return cityName;
    }

    public void setCityName(String cityName) {
        this.cityName = cityName;
    }

    public String getCityKladr() {
        return cityKladr;
    }

    public void setCityKladr(String cityKladr) {
        this.cityKladr = cityKladr;
    }

    public String getCityType() {
        return cityType;
    }

    public void setCityType(String cityType) {
        this.cityType = cityType;
    }

    public String getStreetName() {
        return streetName;
    }

    public void setStreetName(String streetName) {
        this.streetName = streetName;
    }

    public String getStreetKladr() {
        return streetKladr;
    }

    public void setStreetKladr(String streetKladr) {
        this.streetKladr = streetKladr;
    }

    public String getStreetType() {
        return streetType;
    }

    public void setStreetType(String streetType) {
        this.streetType = streetType;
    }
}
