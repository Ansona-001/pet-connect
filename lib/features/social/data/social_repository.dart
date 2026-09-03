import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:uuid/uuid.dart';

import '../../../core/network/api_client.dart';
import '../../../core/network/api_exception.dart';
import '../domain/social_models.dart';

final socialRepositoryProvider = Provider<SocialRepository>((ref) {
  return SocialRepository(ref.watch(apiClientProvider));
});

class SocialSnapshot {
  const SocialSnapshot({
    required this.posts,
    required this.stories,
    required this.pets,
    required this.candidates,
    required this.chats,
  });

  final List<FeedPost> posts;
  final List<PetStory> stories;
  final List<PetProfile> pets;
  final List<PetProfile> candidates;
  final List<ChatPreview> chats;
}

class SwipeOutcome {
  const SwipeOutcome({required this.matchedPet, this.chatId});
  final PetProfile? matchedPet;
  final String? chatId;
}

class SocialRepository {
  SocialRepository(this._api);

  final ApiClient _api;
  String? _currentUserId;
  static const _uuid = Uuid();

  Future<SocialSnapshot> load() async {
    try {
      final petsResponse = await _api.dio.get<Map<String, dynamic>>('/me/pets');
      final petRows = _list(_rawData(petsResponse));
      if (petRows.isNotEmpty) {
        _currentUserId = petRows.first['owner_id']?.toString();
      }
      final pets = petRows.map(_petFromJson).toList();
      final sourcePetId = pets.isEmpty ? null : pets.first.id;
      final responses = await Future.wait<Response<Map<String, dynamic>>>([
        _api.dio.get('/feed'),
        _api.dio.get('/stories'),
        _api.dio.get('/chats'),
        if (sourcePetId != null) _api.dio.get('/pets/$sourcePetId/candidates'),
      ]);
      final posts = _items(responses[0]).map(_postFromJson).toList();
      final stories = _items(responses[1]).map(_storyFromJson).toList();
      final chats = _items(responses[2]).map(_chatFromJson).toList();
      final candidates = sourcePetId == null
          ? <PetProfile>[]
          : _items(responses[3]).map(_petFromJson).toList();
      return SocialSnapshot(
        posts: posts,
        stories: stories,
        pets: pets,
        candidates: candidates,
        chats: chats,
      );
    } catch (error) {
      throw ApiException.from(error);
    }
  }

  Future<void> setPostLiked(String postId, bool value) =>
      _toggle('/posts/$postId/like', value);

  Future<void> setPostSaved(String postId, bool value) =>
      _toggle('/posts/$postId/save', value);

  Future<FeedPost> createImagePost({
    required String petId,
    required String caption,
    required String fileName,
    required List<int> bytes,
    String locationName = '',
  }) async {
    try {
      final extension = fileName.toLowerCase().split('.').last;
      final subtype = extension == 'png'
          ? 'png'
          : extension == 'webp'
          ? 'webp'
          : 'jpeg';
      final uploadResponse = await _api.dio.post<Map<String, dynamic>>(
        '/media/uploads',
        data: FormData.fromMap({
          'file': MultipartFile.fromBytes(
            bytes,
            filename: fileName,
            contentType: DioMediaType('image', subtype),
          ),
        }),
      );
      final upload = _data(uploadResponse);
      final mediaPath = upload['path']?.toString();
      if (mediaPath == null || mediaPath.isEmpty) {
        throw const ApiException('The media upload returned no file path.');
      }
      final postResponse = await _api.dio.post<Map<String, dynamic>>(
        '/posts',
        data: {
          'pet_id': petId,
          'caption': caption.trim(),
          'location_name': locationName.trim(),
          'media_url': mediaPath,
          'media_type': 'image',
          'visibility': 'public',
        },
      );
      return _postFromJson(_data(postResponse));
    } catch (error) {
      throw ApiException.from(error);
    }
  }

  Future<List<String>> comments(String postId) async {
    try {
      final response = await _api.dio.get<Map<String, dynamic>>(
        '/posts/$postId/comments',
      );
      return _items(response)
          .map((item) => item['body']?.toString() ?? '')
          .where((body) => body.isNotEmpty)
          .toList(growable: false);
    } catch (error) {
      throw ApiException.from(error);
    }
  }

  Future<String> addComment(String postId, String body) async {
    try {
      final response = await _api.dio.post<Map<String, dynamic>>(
        '/posts/$postId/comments',
        data: {'body': body.trim()},
      );
      return _data(response)['body']?.toString() ?? body.trim();
    } catch (error) {
      throw ApiException.from(error);
    }
  }

  Future<void> _toggle(String path, bool value) async {
    try {
      if (value) {
        await _api.dio.put<void>(path);
      } else {
        await _api.dio.delete<void>(path);
      }
    } catch (error) {
      throw ApiException.from(error);
    }
  }

  Future<SwipeOutcome> swipe({
    required String sourcePetId,
    required PetProfile target,
    required String decision,
  }) async {
    try {
      final response = await _api.dio.post<Map<String, dynamic>>(
        '/pets/$sourcePetId/swipes',
        data: {
          'target_pet_id': target.id,
          'decision': decision,
          'client_request_id': _uuid.v4(),
        },
      );
      final result = _data(response);
      if (result['matched'] != true) {
        return const SwipeOutcome(matchedPet: null);
      }
      final match = _map(result['match']);
      return SwipeOutcome(
        matchedPet: PetProfile(
          id: match['other_pet_id']?.toString() ?? target.id,
          name: match['other_pet_name']?.toString() ?? target.name,
          ownerName: match['other_owner_name']?.toString() ?? target.ownerName,
          breed: match['other_pet_breed']?.toString() ?? target.breed,
          age: match['other_pet_age_label']?.toString() ?? target.age,
          gender: match['other_pet_gender']?.toString() ?? target.gender,
          distanceKm: target.distanceKm,
          imageAsset: _media(
            match['other_pet_image_url']?.toString() ?? target.imageAsset,
          ),
          personality: target.personality,
          bio: target.bio,
          isVerified: target.isVerified,
        ),
        chatId: match['chat_id']?.toString(),
      );
    } catch (error) {
      throw ApiException.from(error);
    }
  }

  Future<List<ChatMessage>> messages(String chatId) async {
    try {
      final response = await _api.dio.get<Map<String, dynamic>>(
        '/chats/$chatId/messages',
      );
      return _items(response).map(_messageFromJson).toList().reversed.toList();
    } catch (error) {
      throw ApiException.from(error);
    }
  }

  Future<ChatMessage> sendMessage(String chatId, String text) async {
    try {
      final response = await _api.dio.post<Map<String, dynamic>>(
        '/chats/$chatId/messages',
        data: {
          'client_message_id': _uuid.v4(),
          'message_type': 'text',
          'body': text,
          'media_url': '',
        },
      );
      return _messageFromJson(_data(response), isMine: true);
    } catch (error) {
      throw ApiException.from(error);
    }
  }

  ChatMessage messageFromRealtime(Map<String, dynamic> data) {
    return _messageFromJson(data);
  }

  Future<void> markRead(String chatId, String messageId) async {
    try {
      await _api.dio.put<void>(
        '/chats/$chatId/read',
        data: {'message_id': messageId},
      );
    } catch (error) {
      throw ApiException.from(error);
    }
  }

  PetProfile _petFromJson(Map<String, dynamic> json) => PetProfile(
    id: json['id']?.toString() ?? '',
    name: json['name']?.toString() ?? 'Pet',
    ownerName: json['owner_name']?.toString() ?? '',
    breed: json['breed']?.toString() ?? json['pet_type']?.toString() ?? '',
    age: json['age_label']?.toString() ?? '',
    gender: json['gender']?.toString() ?? 'unknown',
    distanceKm: _number(json['distance_km']),
    imageAsset: _media(json['primary_image_url']?.toString() ?? ''),
    personality: _strings(json['personality']),
    petType: json['pet_type']?.toString() ?? '',
    interests: _strings(json['interests']),
    bio: json['bio']?.toString() ?? '',
    isVerified: json['is_verified'] == true,
  );

  FeedPost _postFromJson(Map<String, dynamic> json) => FeedPost(
    id: json['id']?.toString() ?? '',
    petName: json['pet_name']?.toString() ?? 'Pet',
    ownerHandle: json['author_name']?.toString() ?? '',
    location: json['location_name']?.toString() ?? '',
    avatarAsset: _media(json['pet_image_url']?.toString() ?? ''),
    mediaAsset: _media(json['media_url']?.toString() ?? ''),
    caption: json['caption']?.toString() ?? '',
    postedAgo: _timeLabel(json['created_at']),
    likes: (json['like_count'] as num?)?.toInt() ?? 0,
    comments: (json['comment_count'] as num?)?.toInt() ?? 0,
    isLiked: json['liked_by_me'] == true,
    isSaved: json['saved_by_me'] == true,
  );

  PetStory _storyFromJson(Map<String, dynamic> json) => PetStory(
    id: json['id']?.toString() ?? '',
    petName: json['pet_name']?.toString() ?? 'Pet',
    imageAsset: _media(json['media_url']?.toString() ?? ''),
  );

  ChatPreview _chatFromJson(Map<String, dynamic> json) {
    final lastMessage = json['last_message'];
    final last = lastMessage is Map
        ? Map<String, dynamic>.from(lastMessage)
        : null;
    return ChatPreview(
      id: json['id']?.toString() ?? '',
      petName: json['other_pet_name']?.toString() ?? 'Pet',
      ownerName: json['other_owner_name']?.toString() ?? '',
      imageAsset: _media(json['other_pet_image_url']?.toString() ?? ''),
      lastMessage: last?['body']?.toString() ?? 'You matched — say hello!',
      time: _timeLabel(last?['created_at'] ?? json['matched_at']),
      unreadCount: (json['unread_count'] as num?)?.toInt() ?? 0,
    );
  }

  ChatMessage _messageFromJson(Map<String, dynamic> json, {bool? isMine}) =>
      ChatMessage(
        id: json['id']?.toString() ?? '',
        text: json['body']?.toString() ?? '',
        time: _timeLabel(json['created_at']),
        isMine: isMine ?? json['sender_user_id']?.toString() == _currentUserId,
      );

  String _media(String value) => _api.config.resolveMediaUrl(value);

  static Map<String, dynamic> _data(Response<Map<String, dynamic>> response) {
    return _map(_rawData(response));
  }

  static Object? _rawData(Response<Map<String, dynamic>> response) =>
      response.data?['data'];

  static List<Map<String, dynamic>> _items(
    Response<Map<String, dynamic>> response,
  ) {
    final data = _data(response);
    return _list(data['items']);
  }

  static Map<String, dynamic> _map(Object? value) {
    if (value is Map<String, dynamic>) return value;
    if (value is Map) return Map<String, dynamic>.from(value);
    throw const ApiException('The server returned an invalid response.');
  }

  static List<Map<String, dynamic>> _list(Object? value) {
    if (value is! List) return <Map<String, dynamic>>[];
    return value.map(_map).toList(growable: false);
  }

  static List<String> _strings(Object? value) {
    if (value is! List) return <String>[];
    return value.map((item) => item.toString()).toList(growable: false);
  }

  static double _number(Object? value) => (value as num?)?.toDouble() ?? 0;

  static String _timeLabel(Object? value) {
    final time = DateTime.tryParse(value?.toString() ?? '')?.toLocal();
    if (time == null) return '';
    final difference = DateTime.now().difference(time);
    if (difference.inMinutes < 1) return 'Now';
    if (difference.inHours < 1) return '${difference.inMinutes}m';
    if (difference.inDays < 1) return '${difference.inHours}h';
    return '${difference.inDays}d';
  }
}
