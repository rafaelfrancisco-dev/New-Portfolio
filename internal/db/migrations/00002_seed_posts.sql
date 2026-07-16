-- +goose Up
INSERT INTO posts (slug, title, category, teaser, lede, body_md, store_url, published_at) VALUES
(
    'photo-location-recorder',
    'Photo Location Recorder: matching photos to places, on-device',
    'software',
    'Why I built a GPS timeline that never leaves your phone, and how SwiftData made the storage layer almost boring.',
    'My camera takes better photos than my phone, but it has no idea where it is. This is the story of the small app I built to fix that — without sending a single coordinate to a server.',
    'Every landscape photographer with a dedicated camera knows the ritual: you come home with three hundred frames and no memory of which ridge, which bend of the river, which nameless dirt road they came from. GPS add-ons are expensive and eat batteries. Phone apps that track you all day usually mean someone else''s cloud.

So the requirements wrote themselves: record a location timeline all day, keep it entirely on the device, and sip — not drink — the battery. SwiftData handles storage, and significant-location-change monitoring does the heavy lifting, waking the app only when it matters.

<figure>
<div class="placeholder">[ screenshot — timeline view of the app ]</div>
<figcaption>The timeline view: a day of walking, three battery percent.</figcaption>
</figure>

The interesting part was the matching. Camera clocks drift, time zones lie, and daylight saving exists purely to ruin metadata. The importer cross-references EXIF timestamps against the timeline with a configurable tolerance window:

```swift
let window = TimeInterval(tolerance * 60)
let match = timeline.nearest(to: exif.date,
                             within: window)
photo.coordinate = match?.coordinate
```

It shipped this spring. It will never trend on the App Store, and that''s fine — it solves one problem completely, which is the only kind of app I know how to enjoy building.',
    '#',
    '2026-06-21 09:00:00'
),
(
    'griffon-vultures-douro',
    'A spring morning photographing griffon vultures',
    'photography',
    'Four hours in a hide above the Douro, one thermos of coffee, and the frame I''d been chasing for two years.',
    'Four hours in a hide above the Douro, one thermos of coffee, and the frame I''d been chasing for two years.',
    'The hide sits on a cliff edge above the Douro, and you have to be inside it before first light — vultures notice a door closing from a kilometre away. So the morning starts at four, with coffee poured in the dark and a lens cap lost somewhere under the car seat.

The birds arrived with the first thermals, two hours later. Griffons don''t flap if they can help it; they materialise at eye level, wings locked, close enough to hear the air tearing over the primaries. For two years I''d wanted a frame of one banking against the river far below, and for two years they''d banked the other way.

This time one didn''t. One frame, sharp, the Douro a dark ribbon underneath. The other three hundred exposures were the price of admission, and I''d pay it again next week.',
    '',
    '2026-05-09 09:00:00'
),
(
    'five-years-bird-sound-identifier',
    'Five years of Bird Sound Identifier',
    'software',
    '2,100 sales later — what a niche ML app taught me about pricing, reviews, and listening to users (and birds).',
    '2,100 sales later — what a niche ML app taught me about pricing, reviews, and listening to users (and birds).',
    'Bird Sound Identifier turned five this spring: 2,100 sales, 290 reviews, a 4.8★ average, and a support inbox that is mostly people sending me recordings of very confused starlings.

The lessons were not the ones I expected. Pricing mattered less than I feared — the people who want on-device birdsong recognition will pay for it, and the ones who don''t were never customers. Reviews taught me more than analytics ever did: almost every feature worth shipping started as a one-star complaint written at 6 a.m. from a forest somewhere.

And the model is the least interesting part. BirdNET does the heavy lifting; the work is everything around it — microphone handling in wind, offline species ranges, making a result you can trust when the app says it heard a wryneck. Five years in, that''s still the job: not identifying birds, but earning the benefit of the doubt.',
    '',
    '2026-04-14 09:00:00'
),
(
    'winter-fog-mondego-valley',
    'Winter fog in the Mondego valley',
    'photography',
    'Chasing inversion layers before sunrise — and why the best photos happened after I thought the fog had gone.',
    'Chasing inversion layers before sunrise — and why the best photos happened after I thought the fog had gone.',
    'Inversion fog in the Mondego valley is a forecastable miracle: a cold clear night, no wind, and by dawn the river valley is a white sea with the schist ridges as islands. I''d been watching the charts all week, and drove up in the dark to a viewpoint above Penacova.

Sunrise was fine. Postcard-fine, which is the problem — everyone has that photo. By nine the fog was visibly dying and I''d already packed the tripod when the wind shifted and started tearing the layer apart, dragging streamers of it up the side valleys through the pines.

That last half hour was the shoot. Fog as texture instead of blanket, trees appearing and disappearing, light doing something different every ninety seconds. The lesson keeps being the same one: the conditions you came for are rarely the photo you leave with. Stay past the ending.',
    '',
    '2026-02-02 09:00:00'
);

-- +goose Down
DELETE FROM posts WHERE slug IN (
    'photo-location-recorder',
    'griffon-vultures-douro',
    'five-years-bird-sound-identifier',
    'winter-fog-mondego-valley'
);
