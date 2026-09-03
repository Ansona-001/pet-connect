import '../domain/social_models.dart';

class MockSocialData {
  MockSocialData._();

  static const String golden = 'assets/images/onboarding/onboarding_1.jpg';
  static const String sheltie = 'assets/images/onboarding/onboarding_2.jpg';
  static const String cat = 'assets/images/onboarding/onboarding_3.jpg';
  static const String playdate = 'assets/images/content/pet_playdate.png';

  static const stories = <PetStory>[
    PetStory(id: 'own', petName: 'Your story', imageAsset: golden, isOwn: true),
    PetStory(id: 'story-luna', petName: 'Luna', imageAsset: golden),
    PetStory(id: 'story-milo', petName: 'Milo', imageAsset: sheltie),
    PetStory(id: 'story-simba', petName: 'Simba', imageAsset: cat),
    PetStory(
      id: 'story-coco',
      petName: 'Coco',
      imageAsset: playdate,
      isViewed: true,
    ),
  ];

  static const posts = <FeedPost>[
    FeedPost(
      id: 'post-playdate',
      petName: 'Luna & Coco',
      ownerHandle: '@luna.and.friends',
      location: 'Al Barsha Pond Park',
      avatarAsset: golden,
      mediaAsset: playdate,
      caption:
          'When a quick hello turns into the best playdate ever. Same time tomorrow? 🐾',
      postedAgo: '18m',
      likes: 284,
      comments: 31,
    ),
    FeedPost(
      id: 'post-milo',
      petName: 'Milo',
      ownerHandle: '@milo.explores',
      location: 'Mushrif Park',
      avatarAsset: sheltie,
      mediaAsset: sheltie,
      caption:
          'Fresh grass, open trails, and absolutely no intention of going home.',
      postedAgo: '1h',
      likes: 912,
      comments: 68,
    ),
    FeedPost(
      id: 'post-simba',
      petName: 'Simba',
      ownerHandle: '@simba.the.gentleman',
      location: 'Downtown Dubai',
      avatarAsset: cat,
      mediaAsset: cat,
      caption: 'Today’s agenda: one cuddle, three naps, zero meetings.',
      postedAgo: '3h',
      likes: 1408,
      comments: 104,
    ),
  ];

  static const matchCandidates = <PetProfile>[
    PetProfile(
      id: 'coco',
      name: 'Coco',
      ownerName: 'Maya',
      breed: 'Pembroke Corgi',
      age: '2 yrs',
      gender: 'Female',
      distanceKm: 1.2,
      imageAsset: playdate,
      personality: ['Playful', 'Social', 'Gentle'],
      bio: 'Tiny legs, huge park energy. Always ready to make a new friend.',
      isVerified: true,
      isOnline: true,
    ),
    PetProfile(
      id: 'milo',
      name: 'Milo',
      ownerName: 'Noah',
      breed: 'Shetland Sheepdog',
      age: '3 yrs',
      gender: 'Male',
      distanceKm: 2.8,
      imageAsset: sheltie,
      personality: ['Energetic', 'Friendly', 'Curious'],
      bio: 'Trail runner, ball collector, and certified good boy.',
      isVerified: true,
    ),
    PetProfile(
      id: 'simba',
      name: 'Simba',
      ownerName: 'Sara',
      breed: 'British Shorthair',
      age: '4 yrs',
      gender: 'Male',
      distanceKm: 4.1,
      imageAsset: cat,
      personality: ['Calm', 'Cuddly', 'Independent'],
      bio: 'Window watcher and professional slow-blinker.',
      isOnline: true,
    ),
  ];

  static const chats = <ChatPreview>[
    ChatPreview(
      id: 'chat-coco',
      petName: 'Coco',
      ownerName: 'Maya',
      imageAsset: playdate,
      lastMessage: 'Coco would love Saturday morning! 🐾',
      time: '2m',
      unreadCount: 2,
      isOnline: true,
    ),
    ChatPreview(
      id: 'chat-milo',
      petName: 'Milo',
      ownerName: 'Noah',
      imageAsset: sheltie,
      lastMessage: 'That trail looks perfect.',
      time: '1h',
      isOnline: true,
    ),
    ChatPreview(
      id: 'chat-simba',
      petName: 'Simba',
      ownerName: 'Sara',
      imageAsset: cat,
      lastMessage: 'Sent a photo',
      time: 'Yesterday',
    ),
  ];

  static const messages = <ChatMessage>[
    ChatMessage(
      id: 'm1',
      text: 'Hey! Coco had such a good time with Luna today.',
      time: '4:18 PM',
      isMine: false,
    ),
    ChatMessage(
      id: 'm2',
      text: 'Same here! Luna has been asleep since we got home 😄',
      time: '4:20 PM',
      isMine: true,
    ),
    ChatMessage(
      id: 'm3',
      text: 'Want to meet at the small-dog lawn this Saturday?',
      time: '4:21 PM',
      isMine: false,
    ),
    ChatMessage(
      id: 'm4',
      text: 'Absolutely. How about 9:30?',
      time: '4:22 PM',
      isMine: true,
    ),
    ChatMessage(
      id: 'm5',
      text: 'Coco would love Saturday morning! 🐾',
      time: '4:23 PM',
      isMine: false,
    ),
  ];

  static const communities = <CommunityPreview>[
    CommunityPreview(
      name: 'Dubai Dog Parents',
      members: '12.4k members',
      icon: '🐕',
      colorValue: 0xFF5B4BFF,
    ),
    CommunityPreview(
      name: 'Cat People UAE',
      members: '8.1k members',
      icon: '🐈',
      colorValue: 0xFFFF7A59,
    ),
    CommunityPreview(
      name: 'Weekend Walkies',
      members: '4.7k members',
      icon: '🌿',
      colorValue: 0xFF2ECC71,
    ),
  ];

  static const events = <EventPreview>[
    EventPreview(
      title: 'Sunset social walk',
      dateLabel: 'SAT · 5:30 PM',
      location: 'Dubai Hills Dog Park',
      attendees: 42,
    ),
    EventPreview(
      title: 'Adoption open day',
      dateLabel: 'SUN · 10:00 AM',
      location: 'The Petshop, DIP',
      attendees: 76,
    ),
  ];
}
