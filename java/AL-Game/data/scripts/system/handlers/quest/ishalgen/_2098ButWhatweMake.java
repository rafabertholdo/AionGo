/*
 * This file is part of aion-unique <aion-unique.org>.
 *
 *  aion-unique is free software: you can redistribute it and/or modify
 *  it under the terms of the GNU General Public License as published by
 *  the Free Software Foundation, either version 3 of the License, or
 *  (at your option) any later version.
 *
 *  aion-unique is distributed in the hope that it will be useful,
 *  but WITHOUT ANY WARRANTY; without even the implied warranty of
 *  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 *  GNU General Public License for more details.
 *
 *  You should have received a copy of the GNU General Public License
 *  along with aion-unique.  If not, see <http://www.gnu.org/licenses/>.
 */
package quest.ishalgen;

import com.aionemu.gameserver.model.gameobjects.Npc;
import com.aionemu.gameserver.model.gameobjects.player.Player;
import com.aionemu.gameserver.network.aion.serverpackets.SM_DIALOG_WINDOW;
import com.aionemu.gameserver.questEngine.handlers.QuestHandler;
import com.aionemu.gameserver.questEngine.model.QuestEnv;
import com.aionemu.gameserver.questEngine.model.QuestState;
import com.aionemu.gameserver.questEngine.model.QuestStatus;
import com.aionemu.gameserver.services.QuestService;
import com.aionemu.gameserver.utils.PacketSendUtility;

/**
 * @author Manu72 recode dune11
 * 
 */
public class _2098ButWhatweMake extends QuestHandler
{

	private final static int	questId	= 2098;

	public _2098ButWhatweMake()
	{
		super(questId);
	}

	@Override
	public void register()
	{
		qe.addQuestLvlUp(questId);
		qe.setNpcQuestData(203550).addOnTalkEvent(questId); // Munin
		qe.setNpcQuestData(204361).addOnTalkEvent(questId); // Hreidmar
		qe.setNpcQuestData(204408).addOnTalkEvent(questId); // Bulagan
		qe.setNpcQuestData(205198).addOnTalkEvent(questId); // Cayron
		qe.setNpcQuestData(204805).addOnTalkEvent(questId); // Vanargand
		qe.setNpcQuestData(204808).addOnTalkEvent(questId); // Esnu
		qe.setNpcQuestData(203546).addOnTalkEvent(questId); // Skuld
		qe.setNpcQuestData(204387).addOnTalkEvent(questId); // Ananta
		qe.setNpcQuestData(205190).addOnTalkEvent(questId); // Seznec
		qe.setNpcQuestData(204207).addOnTalkEvent(questId); // Kasir
		qe.setNpcQuestData(204301).addOnTalkEvent(questId); // Aegir
		qe.setNpcQuestData(205155).addOnTalkEvent(questId); // Heintz
		qe.setNpcQuestData(204784).addOnTalkEvent(questId); // Delris
		qe.setNpcQuestData(278001).addOnTalkEvent(questId); // Votan
		qe.setNpcQuestData(204053).addOnTalkEvent(questId); // Kvasir
	}

	@Override
	public boolean onLvlUpEvent(QuestEnv env)
	{
		Player player = env.getPlayer();
		QuestState qs = player.getQuestStateList().getQuestState(questId);
		if(qs != null)
			return false;
		boolean lvlCheck = QuestService.checkLevelRequirement(questId, player.getCommonData().getLevel());
		if(!lvlCheck)
			return false;
		env.setQuestId(questId);
		QuestService.startQuest(env, QuestStatus.START);
		return true;
	}

	@Override
	public boolean onDialogEvent(QuestEnv env)
	{
		final Player player = env.getPlayer();
		final QuestState qs = player.getQuestStateList().getQuestState(questId);
		if(qs == null)
			return false;

		int var = qs.getQuestVars().getQuestVars();
		int targetId = 0;
		if(env.getVisibleObject() instanceof Npc)
			targetId = ((Npc) env.getVisibleObject()).getNpcId();

		if(qs.getStatus() == QuestStatus.START)
		{
			switch (targetId)
			{
				case 203550:  //MUNIN
					switch(env.getDialogId())
					{
						case 25:
							if(var == 0)
								return sendQuestDialog(player, env.getVisibleObject().getObjectId(), 1011);
						case 10000:
							if(var == 0)
							{
								qs.setQuestVarById(0, var + 1);
								updateQuestStatus(player, qs);
								PacketSendUtility.sendPacket(player, new SM_DIALOG_WINDOW(env.getVisibleObject().getObjectId(), 10));
								return true;
							}
					}
					break;
				case 204361: // Hreidmar
					switch(env.getDialogId())
					{
						case 25:
							if(var == 1)
								return sendQuestDialog(player, env.getVisibleObject().getObjectId(), 1352);
						case 10001:
							if(var == 1)
							{
								qs.setQuestVarById(0, var + 1);
								updateQuestStatus(player, qs);
								PacketSendUtility.sendPacket(player, new SM_DIALOG_WINDOW(env.getVisibleObject().getObjectId(), 10));
								return true;
							}
					}
					break;
				case 204408: // Bulagan
					switch(env.getDialogId())
					{
						case 25:
							if(var == 2)
								return sendQuestDialog(player, env.getVisibleObject().getObjectId(), 1693);
							return true;
						case 10002:
							if (var == 2)
							{
								qs.setQuestVarById(0, var + 1);
								updateQuestStatus(player, qs);
								PacketSendUtility.sendPacket(player, new SM_DIALOG_WINDOW(env.getVisibleObject().getObjectId(), 10));
								return true;
							}	
					}	
					break;
				case 205198: // Cayron
					switch(env.getDialogId())
					{
						case 25:
							if(var == 3)
								return sendQuestDialog(player, env.getVisibleObject().getObjectId(), 2034);
							return true;
						case 10003:
							if (var == 3)
							{
								qs.setQuestVarById(0, var + 1);
								updateQuestStatus(player, qs);
								PacketSendUtility.sendPacket(player, new SM_DIALOG_WINDOW(env.getVisibleObject().getObjectId(), 10));
								return true;
							}	
					}	
					break;
				case 204805: // Vanargand
					switch(env.getDialogId())
					{
						case 25:
							if(var == 4)
								return sendQuestDialog(player, env.getVisibleObject().getObjectId(), 2375);
							return true;
						case 10004:
							if (var == 4)
							{
								qs.setQuestVarById(0, var + 1);
								updateQuestStatus(player, qs);
								PacketSendUtility.sendPacket(player, new SM_DIALOG_WINDOW(env.getVisibleObject().getObjectId(), 10));
								return true;
							}	
					}	
					break;
				case 204808: // Esnu
					switch(env.getDialogId())
					{
						case 25:
							if(var == 5)
								return sendQuestDialog(player, env.getVisibleObject().getObjectId(), 2716);
							return true;
						case 10005:
							if (var == 5)
							{
								qs.setQuestVarById(0, var + 1);
								updateQuestStatus(player, qs);
								PacketSendUtility.sendPacket(player, new SM_DIALOG_WINDOW(env.getVisibleObject().getObjectId(), 10));
								return true;
							}	
					}	
					break;
				case 203546: // Skuld
					switch(env.getDialogId())
					{
						case 25:
							if(var == 6)
								return sendQuestDialog(player, env.getVisibleObject().getObjectId(), 3057);
							return true;
						case 10006:
							if (var == 6)
							{
								qs.setQuestVarById(0, var + 1);
								updateQuestStatus(player, qs);
								PacketSendUtility.sendPacket(player, new SM_DIALOG_WINDOW(env.getVisibleObject().getObjectId(), 10));
								return true;
							}	
					}	
					break;
				case 204387: // Ananta
					switch(env.getDialogId())
					{
						case 25:
							if(var == 7)
								return sendQuestDialog(player, env.getVisibleObject().getObjectId(), 3398);
							return true;
						case 10007:
							if (var == 7)
							{
								qs.setQuestVarById(0, var + 1);
								updateQuestStatus(player, qs);
								PacketSendUtility.sendPacket(player, new SM_DIALOG_WINDOW(env.getVisibleObject().getObjectId(), 10));
								return true;
							}	
					}	
					break;
				case 205190: // Seznec
					switch(env.getDialogId())
					{
						case 25:
							if(var == 8)
								return sendQuestDialog(player, env.getVisibleObject().getObjectId(), 3739);
							return true;
						case 10008:
							if (var == 8)
							{
								qs.setQuestVarById(0, var + 1);
								updateQuestStatus(player, qs);
								PacketSendUtility.sendPacket(player, new SM_DIALOG_WINDOW(env.getVisibleObject().getObjectId(), 10));
								return true;
							}	
					}	
					break;
				case 204207: // Kasir
					switch(env.getDialogId())
					{
						case 25:
							if(var == 9)
								return sendQuestDialog(player, env.getVisibleObject().getObjectId(), 4080);
							return true;
						case 10009:
							if (var == 9)
							{
								qs.setQuestVarById(0, var + 1);
								updateQuestStatus(player, qs);
								PacketSendUtility.sendPacket(player, new SM_DIALOG_WINDOW(env.getVisibleObject().getObjectId(), 10));
								return true;
							}	
					}	
					break;
				case 204301: // Aegir
					switch(env.getDialogId())
					{
						case 25:
							if(var == 10)
								return sendQuestDialog(player, env.getVisibleObject().getObjectId(), 1608);
							return true;
						case 10010:
							if (var == 10)
							{
								qs.setQuestVarById(0, var + 1);
								updateQuestStatus(player, qs);
								PacketSendUtility.sendPacket(player, new SM_DIALOG_WINDOW(env.getVisibleObject().getObjectId(), 10));
								return true;
							}	
					}	
					break;
				case 205155: // Heintz
					switch(env.getDialogId())
					{
						case 25:
							if(var == 11)
								return sendQuestDialog(player, env.getVisibleObject().getObjectId(), 1949);
							return true;
						case 10011:
							if (var == 11)
							{
								qs.setQuestVarById(0, var + 1);
								updateQuestStatus(player, qs);
								PacketSendUtility.sendPacket(player, new SM_DIALOG_WINDOW(env.getVisibleObject().getObjectId(), 10));
								return true;
							}	
					}	
					break;
				case 204784: // Delris
					switch(env.getDialogId())
					{
						case 25:
							if(var == 12)
								return sendQuestDialog(player, env.getVisibleObject().getObjectId(), 2290);
							return true;
						case 10012:
							if (var == 12)
							{
								qs.setQuestVarById(0, var + 1);
								updateQuestStatus(player, qs);
								PacketSendUtility.sendPacket(player, new SM_DIALOG_WINDOW(env.getVisibleObject().getObjectId(), 10));
								return true;
							}	
					}	
					break;
				case 278001: // Votan
					switch(env.getDialogId())
					{
						case 25:
							if(var == 13)
								return sendQuestDialog(player, env.getVisibleObject().getObjectId(), 2631);
							return true;
						case 10013:
							if (var == 13)
							{
								qs.setQuestVarById(0, var + 1);
								updateQuestStatus(player, qs);
								PacketSendUtility.sendPacket(player, new SM_DIALOG_WINDOW(env.getVisibleObject().getObjectId(), 10));
								return true;
							}	
					}	
					break;
				case 204053: // Kvasir
					switch(env.getDialogId())
					{
						case 25:
							if(var == 14)
								return sendQuestDialog(player, env.getVisibleObject().getObjectId(), 2972);
							return true;
						case 10255:
							if (var==14)
							{
								qs.setStatus(QuestStatus.REWARD);
								updateQuestStatus(player, qs);
								PacketSendUtility.sendPacket(player, new SM_DIALOG_WINDOW(env.getVisibleObject().getObjectId(), 10));
							}	
					}	
					break;
					}
		}
		else if (qs.getStatus() == QuestStatus.REWARD && targetId == 203550)
			return defaultQuestEndDialog(env);
		return false;
	}

}
