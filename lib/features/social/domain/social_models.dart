class PetProfile {
  const PetProfile({
    required this.id,
    required this.name,
    required this.ownerName,
    required this.breed,
    required this.age,
    required this.gender,
    required this.distanceKm,
    required this.imageAsset,
    required this.personality,
    this.petType = '',
    this.interests = const [],
    this.bio = '',
    this.isVerified = false,
    this.isOnline = false,
  });

  final String id;
  final String name;
  final String ownerName;
  final String breed;
  final String age;
  final String gender;
  final double distanceKm;
  final String imageAsset;
  final List<String> personality;
  final String petType;
  final List<String> interests;
  final String bio;
  final bool isVerified;
  final bool isOnline;
}

class UserSetupProfile {
  const UserSetupProfile({
    required this.ownerName,
    required this.city,
    required this.petName,
    required this.petType,
    required this.interests,
  });

  const UserSetupProfile.demo()
    : ownerName = 'Alex',
      city = 'Dubai',
      petName = 'Luna',
      petType = 'Dog',
      interests = const ['Playdates', 'Parks', 'Training'];

  final String ownerName;
  final String city;
  final String petName;
  final String petType;
  final List<String> interests;
}

class PetStory {
  const PetStory({
    required this.id,
    required this.petName,
    required this.imageAsset,
    this.isViewed = false,
    this.isOwn = false,
  });

  final String id;
  final String petName;
  final String imageAsset;
  final bool isViewed;
  final bool isOwn;
}

class FeedPost {
  const FeedPost({
    required this.id,
    required this.petName,
    required this.ownerHandle,
    required this.location,
    required this.avatarAsset,
    required this.mediaAsset,
    required this.caption,
    required this.postedAgo,
    required this.likes,
    required this.comments,
    this.isLiked = false,
    this.isSaved = false,
  });

  final String id;
  final String petName;
  final String ownerHandle;
  final String location;
  final String avatarAsset;
  final String mediaAsset;
  final String caption;
  final String postedAgo;
  final int likes;
  final int comments;
  final bool isLiked;
  final bool isSaved;

  FeedPost copyWith({int? likes, int? comments, bool? isLiked, bool? isSaved}) {
    return FeedPost(
      id: id,
      petName: petName,
      ownerHandle: ownerHandle,
      location: location,
      avatarAsset: avatarAsset,
      mediaAsset: mediaAsset,
      caption: caption,
      postedAgo: postedAgo,
      likes: likes ?? this.likes,
      comments: comments ?? this.comments,
      isLiked: isLiked ?? this.isLiked,
      isSaved: isSaved ?? this.isSaved,
    );
  }
}

class ChatPreview {
  const ChatPreview({
    required this.id,
    required this.petName,
    required this.ownerName,
    required this.imageAsset,
    required this.lastMessage,
    required this.time,
    this.unreadCount = 0,
    this.isOnline = false,
  });

  final String id;
  final String petName;
  final String ownerName;
  final String imageAsset;
  final String lastMessage;
  final String time;
  final int unreadCount;
  final bool isOnline;
}

class ChatMessage {
  const ChatMessage({
    required this.id,
    required this.text,
    required this.time,
    required this.isMine,
  });

  final String id;
  final String text;
  final String time;
  final bool isMine;
}

class CommunityPreview {
  const CommunityPreview({
    required this.name,
    required this.members,
    required this.icon,
    required this.colorValue,
  });

  final String name;
  final String members;
  final String icon;
  final int colorValue;
}

class EventPreview {
  const EventPreview({
    required this.title,
    required this.dateLabel,
    required this.location,
    required this.attendees,
  });

  final String title;
  final String dateLabel;
  final String location;
  final int attendees;
}
